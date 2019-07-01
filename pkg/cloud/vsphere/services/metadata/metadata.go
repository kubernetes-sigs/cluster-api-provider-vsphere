/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metadata

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

// New returns the cloud-init metadata as a base-64 encoded string for a given
// machine context.
func New(ctx *context.MachineContext) ([]byte, error) {
	net := ctx.MachineConfig.MachineSpec.Network

	var format string

	// skip network customization for basic DHCP networks reducing the risk of
	// interfaces not coming up.
	// 1) ubuntu 18.04 uses netplan which does not have a fail-safe mode,
	//    if there are any errors configuring the interface, it will stay down
	// 2) ESXi only generates MAC addresses on power on, leaving the configuration
	//    invalid with an empty match MAC Address
	if len(net.Devices) == 1 &&
		len(net.Routes) == 0 &&
		len(net.Devices[0].Nameservers) == 0 &&
		len(net.Devices[0].Routes) == 0 &&
		len(net.Devices[0].SearchDomains) == 0 &&
		len(net.Devices[0].IPAddrs) == 0 {
		format = basic
	} else if ctx.GetSession() != nil && !ctx.GetSession().IsVC() {
		return nil, errors.New("ESXi only supports basic DHCP network configuration")
	} else {
		format = netplan
	}
	buf := &bytes.Buffer{}
	tpl := template.Must(template.New("t").Funcs(
		template.FuncMap{
			"nameservers": func(spec v1alpha1.NetworkDeviceSpec) bool {
				return len(spec.Nameservers) > 0 || len(spec.SearchDomains) > 0
			},
		}).Parse(format))
	if err := tpl.Execute(buf, struct {
		Hostname string
		Devices  []v1alpha1.NetworkDeviceSpec
		Routes   []v1alpha1.NetworkRouteSpec
	}{
		Hostname: ctx.Machine.Name,
		Devices:  net.Devices,
		Routes:   net.Routes,
	}); err != nil {
		return nil, errors.Wrapf(err, "error getting cloud init metadata for machine %q", ctx)
	}
	return buf.Bytes(), nil
}
