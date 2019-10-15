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

package util

import (
	"bytes"
	"context"
	"net"
	"text/template"

	"github.com/pkg/errors"
	vim25types "github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	clusterutilv1 "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// GetMachinesInCluster gets a cluster's Machine resources.
func GetMachinesInCluster(
	ctx context.Context,
	controllerClient client.Client,
	namespace, clusterName string) ([]*clusterv1.Machine, error) {

	labels := map[string]string{clusterv1.MachineClusterLabelName: clusterName}
	machineList := &clusterv1.MachineList{}

	if err := controllerClient.List(
		ctx, machineList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, errors.Wrapf(
			err, "error getting machines in cluster %s/%s",
			namespace, clusterName)
	}

	machines := make([]*clusterv1.Machine, len(machineList.Items))
	for i := range machineList.Items {
		machines[i] = &machineList.Items[i]
	}

	return machines, nil
}

// GetVSphereMachinesInCluster gets a cluster's VSphereMachine resources.
func GetVSphereMachinesInCluster(
	ctx context.Context,
	controllerClient client.Client,
	namespace, clusterName string) ([]*infrav1.VSphereMachine, error) {

	labels := map[string]string{clusterv1.MachineClusterLabelName: clusterName}
	machineList := &infrav1.VSphereMachineList{}

	if err := controllerClient.List(
		ctx, machineList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	machines := make([]*infrav1.VSphereMachine, len(machineList.Items))
	for i := range machineList.Items {
		machines[i] = &machineList.Items[i]
	}

	return machines, nil
}

// GetVSphereMachine gets a VSphereMachine resource for the given CAPI Machine.
func GetVSphereMachine(
	ctx context.Context,
	controllerClient client.Client,
	namespace, machineName string) (*infrav1.VSphereMachine, error) {

	machine := &infrav1.VSphereMachine{}
	namespacedName := apitypes.NamespacedName{
		Namespace: namespace,
		Name:      machineName,
	}
	if err := controllerClient.Get(ctx, namespacedName, machine); err != nil {
		return nil, err
	}
	return machine, nil
}

// GetMachineManagedObjectReference returns the managed object reference
// for a VSphereMachine resource.
func GetMachineManagedObjectReference(machine *infrav1.VSphereMachine) vim25types.ManagedObjectReference {
	return vim25types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: machine.Spec.MachineRef,
	}
}

// ErrNoMachineIPAddr indicates that no valid IP addresses were found in a machine context
var ErrNoMachineIPAddr = errors.New("no IP addresses found for machine")

// GetMachinePreferredIPAddress returns the preferred IP address for a
// VSphereMachine resource.
func GetMachinePreferredIPAddress(machine *infrav1.VSphereMachine) (string, error) {
	var cidr *net.IPNet
	if cidrString := machine.Spec.Network.PreferredAPIServerCIDR; cidrString != "" {
		var err error
		if _, cidr, err = net.ParseCIDR(cidrString); err != nil {
			return "", errors.New("error parsing preferred API server CIDR")
		}
	}

	for _, nodeAddr := range machine.Status.Addresses {
		if nodeAddr.Type != corev1.NodeInternalIP {
			continue
		}
		if cidr == nil {
			return nodeAddr.Address, nil
		}
		if cidr.Contains(net.ParseIP(nodeAddr.Address)) {
			return nodeAddr.Address, nil
		}
	}

	return "", ErrNoMachineIPAddr
}

// IsControlPlaneMachine returns a flag indicating whether or not a machine has
// the control plane role.
func IsControlPlaneMachine(machine *clusterv1.Machine) bool {
	return clusterutilv1.IsControlPlaneMachine(machine)
}

// GetMachineMetadata returns the cloud-init metadata as a base-64 encoded
// string for a given VSphereMachine.
func GetMachineMetadata(hostname string, machine infrav1.VSphereMachine, networkStatus ...infrav1.NetworkStatus) ([]byte, error) {
	// Create a copy of the devices and add their MAC addresses from a network status.
	devices := make([]infrav1.NetworkDeviceSpec, len(machine.Spec.Network.Devices))
	for i := range machine.Spec.Network.Devices {
		machine.Spec.Network.Devices[i].DeepCopyInto(&devices[i])
		if len(networkStatus) > 0 {
			devices[i].MACAddr = networkStatus[i].MACAddr
		}
	}

	buf := &bytes.Buffer{}
	tpl := template.Must(template.New("t").Funcs(
		template.FuncMap{
			"nameservers": func(spec infrav1.NetworkDeviceSpec) bool {
				return len(spec.Nameservers) > 0 || len(spec.SearchDomains) > 0
			},
		}).Parse(metadataFormat))
	if err := tpl.Execute(buf, struct {
		Hostname string
		Devices  []infrav1.NetworkDeviceSpec
		Routes   []infrav1.NetworkRouteSpec
	}{
		Hostname: hostname, // note that hostname determines the Kubernetes node name
		Devices:  devices,
		Routes:   machine.Spec.Network.Routes,
	}); err != nil {
		return nil, errors.Wrapf(
			err,
			"error getting cloud init metadata for machine %s/%s/%s",
			machine.Namespace, machine.ClusterName, machine.Name)
	}
	return buf.Bytes(), nil
}
