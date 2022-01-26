/*
Copyright 2021 The Kubernetes Authors.

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
	"fmt"

	"github.com/pkg/errors"
	netopv1 "github.com/vmware-tanzu/net-operator-api/api/v1alpha1"
	vmopv1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	ncpv1 "github.com/vmware-tanzu/vm-operator/external/ncp/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

const (
	NSXTTypeNetwork = "nsx-t"
	// This constant is also defined in VM Operator.
	NSXTVNetSelectorKey = "ncp.vmware.com/virtual-network-name"

	CAPVDefaultNetworkLabel    = "capv.vmware.com/is-default-network"
	NetOpNetworkNameAnnotation = "netoperator.vmware.com/network-name"

	// kube-system network is where supervisor control plane vms reside.
	SystemNamespace = "kube-system"
)

// dummyNetworkProvider doesn't provision network resource.
type dummyNetworkProvider struct{}

// DummyNetworkProvider returns an instance of dummy network provider.
func DummyNetworkProvider() services.NetworkProvider {
	return &dummyNetworkProvider{}
}

func (np *dummyNetworkProvider) HasLoadBalancer() bool {
	return false
}

func (np *dummyNetworkProvider) ProvisionClusterNetwork(ctx *vmware.ClusterContext) error {
	return nil
}

func (np *dummyNetworkProvider) GetClusterNetworkName(ctx *vmware.ClusterContext) (string, error) {
	return "", nil
}

func (np *dummyNetworkProvider) ConfigureVirtualMachine(ctx *vmware.ClusterContext, vm *vmopv1.VirtualMachine) error {
	return nil
}

func (np *dummyNetworkProvider) GetVMServiceAnnotations(ctx *vmware.ClusterContext) (map[string]string, error) {
	return map[string]string{}, nil
}

func (np *dummyNetworkProvider) VerifyNetworkStatus(ctx *vmware.ClusterContext, obj runtime.Object) error {
	return nil
}

type dummyLBNetworkProvider struct {
	dummyNetworkProvider
}

// DummyLBNetworkProvider returns an instance of dummy network provider that has a LB.
func DummyLBNetworkProvider() services.NetworkProvider {
	return &dummyLBNetworkProvider{}
}

func (np *dummyLBNetworkProvider) HasLoadBalancer() bool {
	return true
}

type netopNetworkProvider struct {
	client client.Client
}

func NetOpNetworkProvider(client client.Client) services.NetworkProvider {
	return &netopNetworkProvider{
		client: client,
	}
}

func (np *netopNetworkProvider) HasLoadBalancer() bool {
	return true
}

func (np *netopNetworkProvider) ProvisionClusterNetwork(ctx *vmware.ClusterContext) error {
	conditions.MarkTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)
	return nil
}

func (np *netopNetworkProvider) getDefaultClusterNetwork(ctx *vmware.ClusterContext) (*netopv1.Network, error) {
	labels := map[string]string{CAPVDefaultNetworkLabel: "true"}

	networkList := &netopv1.NetworkList{}
	err := np.client.List(ctx, networkList, client.InNamespace(ctx.Cluster.Namespace), client.MatchingLabels(labels))
	if err != nil {
		return nil, err
	}

	switch len(networkList.Items) {
	case 0:
		return nil, fmt.Errorf("no default CAPV Network found")
	case 1:
		return &networkList.Items[0], nil
	default:
		return nil, fmt.Errorf("more than one default CAPV Network found: %d", len(networkList.Items))
	}
}

func (np *netopNetworkProvider) getClusterNetwork(ctx *vmware.ClusterContext) (*netopv1.Network, error) {
	// A "NetworkName" can later be added to the TKG Spec, but currently we only have a preselected default.
	return np.getDefaultClusterNetwork(ctx)
}

func (np *netopNetworkProvider) GetClusterNetworkName(ctx *vmware.ClusterContext) (string, error) {
	network, err := np.getClusterNetwork(ctx)
	if err != nil {
		return "", err
	}

	return network.Name, nil
}

func (np *netopNetworkProvider) GetVMServiceAnnotations(ctx *vmware.ClusterContext) (map[string]string, error) {
	networkName, err := np.GetClusterNetworkName(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]string{NetOpNetworkNameAnnotation: networkName}, nil
}

func (np *netopNetworkProvider) ConfigureVirtualMachine(ctx *vmware.ClusterContext, vm *vmopv1.VirtualMachine) error {
	network, err := np.getClusterNetwork(ctx)
	if err != nil {
		return err
	}

	for _, vnif := range vm.Spec.NetworkInterfaces {
		if vnif.NetworkType == string(network.Spec.Type) && vnif.NetworkName == network.Name {
			// Expected network interface already exists.
			return nil
		}
	}

	vm.Spec.NetworkInterfaces = append(vm.Spec.NetworkInterfaces, vmopv1.VirtualMachineNetworkInterface{
		NetworkName: network.Name,
		NetworkType: string(network.Spec.Type),
	})

	return nil
}

func (np *netopNetworkProvider) VerifyNetworkStatus(ctx *vmware.ClusterContext, obj runtime.Object) error {
	_, ok := obj.(*netopv1.Network)
	if !ok {
		return fmt.Errorf("expected Net Operator Network but got %T", obj)
	}

	// Network doesn't have a []Conditions but the specific network type pointed to by ProviderRef might.
	// The VSphereDistributedNetwork does but it is not currently populated by net-operator.

	return nil
}

// nsxtNetworkProvider provision nsx-t type cluster network.
type nsxtNetworkProvider struct {
	client    client.Client
	disableFW string // "true" means disable firewalls on GC network
}

// NsxtNetworkProvider returns an instance of nsx-t type network provider.
func NsxtNetworkProvider(client client.Client, disableFW string) services.NetworkProvider {
	return &nsxtNetworkProvider{
		client:    client,
		disableFW: disableFW,
	}
}

func (np *nsxtNetworkProvider) HasLoadBalancer() bool {
	return true
}

// GetNSXTVirtualNetworkName returns the name of the NSX-T vnet object.
func GetNSXTVirtualNetworkName(clusterName string) string {
	return fmt.Sprintf("%s-vnet", clusterName)
}

func (np *nsxtNetworkProvider) verifyNSXTVirtualNetworkStatus(ctx *vmware.ClusterContext, vnet *ncpv1.VirtualNetwork) error {
	clusterName := ctx.VSphereCluster.Name
	namespace := ctx.VSphereCluster.Namespace
	for _, condition := range vnet.Status.Conditions {
		if condition.Type == "Ready" && condition.Status != "True" {
			conditions.MarkFalse(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition, infrav1.ClusterNetworkProvisionFailedReason, clusterv1.ConditionSeverityWarning, condition.Message)
			return errors.Errorf("virtual network ready status is: '%s' in cluster %s. reason: %s, message: %s",
				condition.Status, types.NamespacedName{Namespace: namespace, Name: clusterName}, condition.Reason, condition.Message)
		}
	}

	conditions.MarkTrue(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition)
	return nil
}

func (np *nsxtNetworkProvider) VerifyNetworkStatus(ctx *vmware.ClusterContext, obj runtime.Object) error {
	vnet, ok := obj.(*ncpv1.VirtualNetwork)
	if !ok {
		return fmt.Errorf("expected NCP VirtualNetwork but got %T", obj)
	}

	return np.verifyNSXTVirtualNetworkStatus(ctx, vnet)
}

func (np *nsxtNetworkProvider) ProvisionClusterNetwork(ctx *vmware.ClusterContext) error {
	cluster := ctx.VSphereCluster
	clusterKey := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}

	ctx.Logger.V(2).Info("Provisioning ", "vnet", GetNSXTVirtualNetworkName(cluster.Name), "namespace", cluster.Namespace)
	defer ctx.Logger.V(2).Info("Finished provisioning", "vnet", GetNSXTVirtualNetworkName(cluster.Name), "namespace", cluster.Namespace)

	vnet := &ncpv1.VirtualNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      GetNSXTVirtualNetworkName(cluster.Name),
		},
	}

	_, err := ctrlutil.CreateOrUpdate(ctx, np.client, vnet, func() error {
		// add or update vnet spec only if FW is enabled and if WhitelistSourceRanges is empty
		if np.disableFW != "true" && vnet.Spec.WhitelistSourceRanges == "" {
			supportFW, err := util.NCPSupportFW(ctx, np.client)
			if err != nil {
				ctx.Logger.Error(err, "failed to check if NCP supports firewall rules enforcement on GC T1 router")
				return err
			}
			// specify whitelist_source_ranges if needed and if NCP supports it
			if supportFW {
				// Find system namespace snat ip
				systemNSSnatIP, err := util.GetNamespaceNetSnatIP(ctx, np.client, SystemNamespace)
				if err != nil {
					ctx.Logger.Error(err, "failed to get Snat IP for kube-system")
					return err
				}
				ctx.Logger.V(4).Info("got system namespace snat ip",
					"cluster", clusterKey, "ip", systemNSSnatIP)

				// WhitelistSourceRanges accept cidrs only
				vnet.Spec.WhitelistSourceRanges = systemNSSnatIP + "/32"
			}
		}

		vnet.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "VSphereCluster",
				Name:       cluster.Name,
				UID:        cluster.UID,
			},
		})

		return nil
	})
	if err != nil {
		conditions.MarkFalse(ctx.VSphereCluster, infrav1.ClusterNetworkReadyCondition, infrav1.ClusterNetworkProvisionFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		ctx.Logger.V(2).Info("Failed to provision network", "cluster", clusterKey)
		return err
	}

	return np.verifyNSXTVirtualNetworkStatus(ctx, vnet)
}

// Returns the name of a valid cluster network if one exists.
func (np *nsxtNetworkProvider) GetClusterNetworkName(ctx *vmware.ClusterContext) (string, error) {
	vnet := &ncpv1.VirtualNetwork{}
	cluster := ctx.VSphereCluster
	namespacedName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      GetNSXTVirtualNetworkName(cluster.Name),
	}
	if err := np.client.Get(ctx, namespacedName, vnet); err != nil {
		return "", err
	}
	return namespacedName.Name, nil
}

func (np *nsxtNetworkProvider) GetVMServiceAnnotations(ctx *vmware.ClusterContext) (map[string]string, error) {
	vnetName, err := np.GetClusterNetworkName(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]string{NSXTVNetSelectorKey: vnetName}, nil
}

// ConfigureVirtualMachine configures a VirtualMachine object based on the networking configuration.
func (np *nsxtNetworkProvider) ConfigureVirtualMachine(ctx *vmware.ClusterContext, vm *vmopv1.VirtualMachine) error {
	nsxtClusterNetworkName := GetNSXTVirtualNetworkName(ctx.Cluster.Name)
	for _, vnif := range vm.Spec.NetworkInterfaces {
		if vnif.NetworkType == NSXTTypeNetwork && vnif.NetworkName == nsxtClusterNetworkName {
			// expected network interface is already found
			return nil
		}
	}
	vm.Spec.NetworkInterfaces = append(vm.Spec.NetworkInterfaces, vmopv1.VirtualMachineNetworkInterface{
		NetworkName: nsxtClusterNetworkName,
		NetworkType: NSXTTypeNetwork,
	})
	return nil
}
