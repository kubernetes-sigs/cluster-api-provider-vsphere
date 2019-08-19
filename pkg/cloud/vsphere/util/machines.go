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
	"sort"
	"text/template"

	"github.com/pkg/errors"
	vim25types "github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/pkg/util"
	clusterUtilv1 "sigs.k8s.io/cluster-api/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// GetMachinesInCluster gets a cluster's machine resources.
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
		return nil, err
	}

	machines := make([]*clusterv1.Machine, len(machineList.Items))
	for i := range machineList.Items {
		machines[i] = &machineList.Items[i]
	}

	return machines, nil
}

// GetVSphereMachine gets a VSphereMachine resource for the given CAPI Machine.
func GetVSphereMachine(
	ctx context.Context,
	controllerClient client.Client,
	namespace, machineName string) (*v1alpha2.VSphereMachine, error) {

	machine := &v1alpha2.VSphereMachine{}
	namespacedName := apitypes.NamespacedName{
		Namespace: namespace,
		Name:      machineName,
	}
	if err := controllerClient.Get(ctx, namespacedName, machine); err != nil {
		return nil, err
	}
	return machine, nil
}

// byMachineCreatedTimestamp implements sort.Interface for []clusterv1.Machine
// based on the machine's creation timestamp.
type byMachineCreatedTimestamp []*clusterv1.Machine

func (a byMachineCreatedTimestamp) Len() int      { return len(a) }
func (a byMachineCreatedTimestamp) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byMachineCreatedTimestamp) Less(i, j int) bool {
	return a[i].CreationTimestamp.Before(&a[j].CreationTimestamp)
}

// GetOldestControlPlaneMachine returns the oldest control plane machine in
// the cluster.
func GetOldestControlPlaneMachine(
	ctx context.Context,
	controllerClient client.Client,
	namespace, clusterName string) (*clusterv1.Machine, error) {

	machines, err := GetMachinesInCluster(ctx, controllerClient, namespace, clusterName)
	if err != nil {
		return nil, err
	}

	controlPlaneMachines := util.GetControlPlaneMachines(machines)
	if len(controlPlaneMachines) == 0 {
		return nil, nil
	}

	// Sort the control plane machines so the first one created is always the
	// one used to provide the address for the control plane endpoint.
	sortedControlPlaneMachines := byMachineCreatedTimestamp(controlPlaneMachines)
	sort.Sort(sortedControlPlaneMachines)

	return sortedControlPlaneMachines[0], nil
}

// GetMachineManagedObjectReference returns the managed object reference
// for a VSphereMachine resource.
func GetMachineManagedObjectReference(machine *v1alpha2.VSphereMachine) (*vim25types.ManagedObjectReference, error) {
	if machine.Spec.MachineRef == "" {
		return nil, nil
	}
	return &vim25types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: machine.Spec.MachineRef,
	}, nil
}

// ErrNoMachineIPAddr indicates that no valid IP addresses were found in a machine context
var ErrNoMachineIPAddr = errors.New("no IP addresses found for machine")

// GetMachinePreferredIPAddress returns the preferred IP address for a
// VSphereMachine resource.
func GetMachinePreferredIPAddress(machine *v1alpha2.VSphereMachine) (string, error) {
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
	return clusterUtilv1.IsControlPlaneMachine(machine)
}

// GetMachineMetadata returns the cloud-init metadata as a base-64 encoded
// string for a given VSphereMachine.
func GetMachineMetadata(machine *v1alpha2.VSphereMachine) ([]byte, error) {
	buf := &bytes.Buffer{}
	tpl := template.Must(template.New("t").Funcs(
		template.FuncMap{
			"nameservers": func(spec v1alpha2.NetworkDeviceSpec) bool {
				return len(spec.Nameservers) > 0 || len(spec.SearchDomains) > 0
			},
		}).Parse(metadataFormat))
	if err := tpl.Execute(buf, struct {
		Hostname string
		Devices  []v1alpha2.NetworkDeviceSpec
		Routes   []v1alpha2.NetworkRouteSpec
	}{
		Hostname: machine.Name,
		Devices:  machine.Spec.Network.Devices,
		Routes:   machine.Spec.Network.Routes,
	}); err != nil {
		return nil, errors.Wrapf(
			err,
			"error getting cloud init metadata for machine %s/%s/%s",
			machine.Namespace, machine.ClusterName, machine.Name)
	}
	return buf.Bytes(), nil
}
