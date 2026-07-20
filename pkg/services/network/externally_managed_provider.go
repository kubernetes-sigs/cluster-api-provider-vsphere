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

	"github.com/pkg/errors"
	nsxvpcv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// SupportsIPv6DualStack is unreachable for this provider: the VMService LB path
// and ServiceDiscovery address discovery that consume it are both skipped.
// Dual-stack is instead derived per-interface from the referenced Subnet / SubnetSet.
func (np *externallyManagedNetworkProvider) SupportsIPv6DualStack() bool {
	return false
}

func (np *externallyManagedNetworkProvider) HasLoadBalancer() bool {
	return false
}

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

func (np *externallyManagedNetworkProvider) GetClusterNetworkName(_ context.Context, _ *vmware.ClusterContext) (string, error) {
	return "", nil
}

func (np *externallyManagedNetworkProvider) GetVMServiceAnnotations(_ context.Context, _ *vmware.ClusterContext) (map[string]string, error) {
	return map[string]string{}, nil
}

func (np *externallyManagedNetworkProvider) VerifyNetworkStatus(_ context.Context, _ *vmware.ClusterContext, _ runtime.Object) error {
	return nil
}

// ConfigureVirtualMachine straight-propagates machine interfaces and VLANs onto the VM.
// Interface routes are not propagated. Primary is required (workload network on eth0).
func (np *externallyManagedNetworkProvider) ConfigureVirtualMachine(ctx context.Context, _ *vmware.ClusterContext, machine *vmwarev1.VSphereMachine, vm *vmoprvhub.VirtualMachine) error {
	primary := machine.Spec.Network.Interfaces.Primary
	if !primary.IsDefined() {
		return errors.New("primary interface must be defined")
	}

	vm.Spec.Network = &vmoprvhub.VirtualMachineNetworkSpec{}

	primaryIPAMModes, err := np.ipamModesForInterface(ctx, machine.Namespace, primary.NetworkRef)
	if err != nil {
		return err
	}
	vm.Spec.Network.Interfaces = append(vm.Spec.Network.Interfaces, vmInterfaceFromSpec(
		PrimaryInterfaceName,
		primary,
		"", // gateway4
		"", // gateway6
		primaryIPAMModes,
	))

	for _, secondary := range machine.Spec.Network.Interfaces.Secondary {
		ipamModes, err := np.ipamModesForInterface(ctx, machine.Namespace, secondary.NetworkRef)
		if err != nil {
			return err
		}
		vm.Spec.Network.Interfaces = append(vm.Spec.Network.Interfaces, vmInterfaceFromSpec(
			secondary.Name,
			secondary.InterfaceSpec,
			"None",
			"None",
			ipamModes,
		))
	}

	return setVLANs(machine, vm)
}

func vmInterfaceFromSpec(name string, iface vmwarev1.InterfaceSpec, gateway4, gateway6 string, ipamModes []corev1.IPFamily) vmoprvhub.VirtualMachineNetworkInterfaceSpec {
	var mtu *int64
	if iface.MTU != 0 {
		mtu = ptr.To(int64(iface.MTU))
	}
	return vmoprvhub.VirtualMachineNetworkInterfaceSpec{
		Name: name,
		Network: &vmoprvhub.PartialObjectRef{
			TypeMeta: metav1.TypeMeta{
				Kind:       iface.NetworkRef.Kind,
				APIVersion: iface.NetworkRef.APIVersion,
			},
			Name: iface.NetworkRef.Name,
		},
		MTU:       mtu,
		Gateway4:  gateway4,
		Gateway6:  gateway6,
		IPAMModes: ipamModes,
	}
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
			return nil, errors.Wrapf(err, "failed to get Subnet %s", klog.KRef(namespace, networkRef.Name))
		}
		return ipamModesFromIPAddressType(subnet.Spec.IPAddressType), nil
	case NetworkGVKNSXTVPCSubnetSet:
		subnetSet := &nsxvpcv1.SubnetSet{}
		if err := np.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: networkRef.Name}, subnetSet); err != nil {
			return nil, errors.Wrapf(err, "failed to get SubnetSet %s", klog.KRef(namespace, networkRef.Name))
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
