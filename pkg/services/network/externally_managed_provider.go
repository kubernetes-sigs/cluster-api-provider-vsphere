/*
Copyright 2026 The Kubernetes Authors.

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

package network

import (
	"context"
	"slices"

	pkgerrors "github.com/pkg/errors"
	nsxvpcv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
)

// externallyManagedNetworkProvider propagates VSphereMachine interface specs
// into VirtualMachine network interfaces without provisioning networks or LBs.
type externallyManagedNetworkProvider struct {
	client client.Client
}

// ExternallyManagedNetworkProvider returns an ExternallyManaged network provider.
func ExternallyManagedNetworkProvider(client client.Client) services.NetworkProvider {
	return &externallyManagedNetworkProvider{
		client: client,
	}
}

func (np *externallyManagedNetworkProvider) Name() string {
	return ExternallyManagedNetworkProviderName
}

// SupportsIPv6DualStack reports whether IPv6 dual-stack is enabled. This
// provider derives dual-stack per-interface from Subnet/SubnetSet IPAddressType
// when the IPv6DualStack feature gate is enabled.
func (np *externallyManagedNetworkProvider) SupportsIPv6DualStack() bool {
	return feature.Gates.Enabled(feature.IPv6DualStack)
}

// HasLoadBalancer is false: CAPV will not create a VirtualMachineService for
// the control plane VIP; the Cluster's spec.controlPlaneEndpoint is pre-set.
func (np *externallyManagedNetworkProvider) HasLoadBalancer() bool {
	return false
}

// SupportsVMReadinessProbe is false: no TCP-based readiness probe is configured
// on control plane VMs, as this probe is typically used to perform endpoint
// health checks for the VirtualMachineService-managed Load Balancer.
func (np *externallyManagedNetworkProvider) SupportsVMReadinessProbe() bool {
	return false
}

func (np *externallyManagedNetworkProvider) SupportsSupervisorService() bool {
	return false
}

// ProvisionClusterNetwork creates nothing; network objects are provisioned and
// managed externally, so they are assumed ready. It marks the network ready
// condition True.
func (np *externallyManagedNetworkProvider) ProvisionClusterNetwork(_ context.Context, clusterCtx *vmware.ClusterContext) error {
	deprecatedv1beta1conditions.MarkTrue(clusterCtx.VSphereCluster, vmwarev1.ClusterNetworkReadyV1Beta1Condition)
	conditions.Set(clusterCtx.VSphereCluster, metav1.Condition{
		Type:   vmwarev1.VSphereClusterNetworkReadyCondition,
		Status: metav1.ConditionTrue,
		Reason: vmwarev1.VSphereClusterNetworkReadyReason,
	})
	return nil
}

// GetVMServiceAnnotations returns an empty map. CAPV does not create a
// VirtualMachineService, so these annotations are unused; an empty map keeps
// the call site uniform with other providers.
func (np *externallyManagedNetworkProvider) GetVMServiceAnnotations(_ context.Context, _ *vmware.ClusterContext) (map[string]string, error) {
	return map[string]string{}, nil
}

// ConfigureVirtualMachine straight-propagates machine interfaces and VLANs onto the VM.
// Interface routes are not propagated (and are rejected by the VSphereMachine webhook).
// Primary is required (workload network on eth0).
func (np *externallyManagedNetworkProvider) ConfigureVirtualMachine(ctx context.Context, _ *vmware.ClusterContext, machine *vmwarev1.VSphereMachine, vm *vmoprvhub.VirtualMachine) error {
	primary := machine.Spec.Network.Interfaces.Primary
	if !primary.IsDefined() {
		return pkgerrors.New("primary interface must be defined")
	}

	if vm.Spec.Network == nil {
		vm.Spec.Network = &vmoprvhub.VirtualMachineNetworkSpec{}
	}

	primaryIPAMModes, err := np.ipamModesForInterface(ctx, machine.Namespace, primary.NetworkRef)
	if err != nil {
		return err
	}
	vmInterfaceFromSpec(
		vm.Spec.Network,
		PrimaryInterfaceName,
		primary,
		"", // gateway4
		"", // gateway6
		primaryIPAMModes,
	)

	// interfaceNames records interfaces from the VSphereMachine spec,
	// will remove interfaces in VirtualMachine that are not exist in interfaceNames.
	interfaceNames := map[string]struct{}{PrimaryInterfaceName: {}}
	for _, secondary := range machine.Spec.Network.Interfaces.Secondary {
		ipamModes, err := np.ipamModesForInterface(ctx, machine.Namespace, secondary.NetworkRef)
		if err != nil {
			return err
		}
		vmInterfaceFromSpec(
			vm.Spec.Network,
			secondary.Name,
			secondary.InterfaceSpec,
			"None",
			"None",
			ipamModes,
		)
		interfaceNames[secondary.Name] = struct{}{}
	}

	// Clean up legacy network interfaces
	vm.Spec.Network.Interfaces = slices.DeleteFunc(vm.Spec.Network.Interfaces, func(iface vmoprvhub.VirtualMachineNetworkInterfaceSpec) bool {
		_, desired := interfaceNames[iface.Name]
		return !desired
	})

	// CAPV owns the VLAN list, including removal and links to retained interfaces.
	vm.Spec.Network.VLANs = nil
	return setVLANs(machine, vm)
}

func vmInterfaceFromSpec(network *vmoprvhub.VirtualMachineNetworkSpec, name string, iface vmwarev1.InterfaceSpec, gateway4, gateway6 string, ipamModes []corev1.IPFamily) {
	var vmInterface *vmoprvhub.VirtualMachineNetworkInterfaceSpec

	// use interface name to determine whether already exists
	for i := range network.Interfaces {
		if network.Interfaces[i].Name == name {
			vmInterface = &network.Interfaces[i]
			break
		}
	}

	// new interface and point to vmInterface
	if vmInterface == nil {
		network.Interfaces = append(network.Interfaces, vmoprvhub.VirtualMachineNetworkInterfaceSpec{Name: name})
		vmInterface = &network.Interfaces[len(network.Interfaces)-1]
	}

	vmInterface.Network = &vmoprvhub.PartialObjectRef{
		TypeMeta: metav1.TypeMeta{
			Kind:       iface.NetworkRef.Kind,
			APIVersion: iface.NetworkRef.APIVersion,
		},
		Name: iface.NetworkRef.Name,
	}
	var mtu *int64
	if iface.MTU != 0 {
		mtu = ptr.To(int64(iface.MTU))
	}
	vmInterface.MTU = mtu
	vmInterface.Gateway4 = gateway4
	vmInterface.Gateway6 = gateway6
	vmInterface.IPAMModes = ipamModes
}

// ipamModesForInterface derives IPAMModes from a referenced Subnet or SubnetSet
// when the IPv6DualStack feature gate is enabled. Other network refs leave IPAMModes unset.
func (np *externallyManagedNetworkProvider) ipamModesForInterface(ctx context.Context, namespace string, networkRef vmwarev1.InterfaceNetworkReference) ([]corev1.IPFamily, error) {
	if !feature.Gates.Enabled(feature.IPv6DualStack) {
		return nil, nil
	}

	gvk := networkRef.GroupVersionKind()
	switch gvk {
	case NetworkGVKNSXTVPCSubnet:
		subnet := &nsxvpcv1.Subnet{}
		if err := np.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: networkRef.Name}, subnet); err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to get Subnet %s", klog.KRef(namespace, networkRef.Name))
		}
		return ipamModesFromIPAddressType(subnet.Spec.IPAddressType), nil
	case NetworkGVKNSXTVPCSubnetSet:
		subnetSet := &nsxvpcv1.SubnetSet{}
		if err := np.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: networkRef.Name}, subnetSet); err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to get SubnetSet %s", klog.KRef(namespace, networkRef.Name))
		}
		return ipamModesFromIPAddressType(subnetSet.Spec.IPAddressType), nil
	default:
		return nil, nil
	}
}

func ipamModesFromIPAddressType(ipAddressType nsxvpcv1.IPAddressType) []corev1.IPFamily {
	switch ipAddressType {
	case nsxvpcv1.IPAddressTypeIPv6:
		return []corev1.IPFamily{corev1.IPv6Protocol}
	case nsxvpcv1.IPAddressTypeIPv4IPv6:
		return []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}
	case nsxvpcv1.IPAddressTypeIPv4, "":
		// Empty/unset defaults to IPv4 per the Subnet/SubnetSet CRD default.
		return []corev1.IPFamily{corev1.IPv4Protocol}
	default:
		return []corev1.IPFamily{corev1.IPv4Protocol}
	}
}
