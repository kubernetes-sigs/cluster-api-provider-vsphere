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

package vmoperator

import (
	"context"
	"fmt"
	"sort"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
)

// RPService represents the ability to reconcile a VirtualMachineSetResourcePolicy via vmoperator.
type RPService struct {
	Client client.Client
}

// ReconcileResourcePolicy ensures that a VirtualMachineSetResourcePolicy exists for the cluster
// Returns the name of a policy if it exists, otherwise returns an error.
func (s *RPService) ReconcileResourcePolicy(ctx context.Context, cluster *clusterv1.Cluster, vSphereCluster *vmwarev1.VSphereCluster) (string, error) {
	clusterModuleGroups, err := getTargetClusterModuleGroups(ctx, s.Client, cluster, vSphereCluster)
	if err != nil {
		return "", err
	}

	resourcePolicyName := cluster.Name
	resourcePolicy := &vmoprv1.VirtualMachineSetResourcePolicy{}

	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: resourcePolicyName}, resourcePolicy); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", errors.Wrap(err, "failed to get existing VirtualMachineSetResourcePolicy")
		}

		resourcePolicy = &vmoprv1.VirtualMachineSetResourcePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      resourcePolicyName,
			},
		}

		if err := s.mutateResourcePolicy(resourcePolicy, clusterModuleGroups, cluster, vSphereCluster, true); err != nil {
			return "", errors.Wrap(err, "failed to mutate VirtualMachineSetResourcePolicy")
		}

		if err := s.Client.Create(ctx, resourcePolicy); err != nil {
			return "", errors.Wrap(err, "failed to create VirtualMachineSetResourcePolicy")
		}

		return resourcePolicyName, nil
	}

	// Ensure .spec.clusterModuleGroups is up to date.
	helper, err := patch.NewHelper(resourcePolicy, s.Client)
	if err != nil {
		return "", err
	}

	if err := s.mutateResourcePolicy(resourcePolicy, clusterModuleGroups, cluster, vSphereCluster, false); err != nil {
		return "", errors.Wrap(err, "failed to mutate VirtualMachineSetResourcePolicy")
	}

	resourcePolicy.Spec.ClusterModuleGroups = clusterModuleGroups
	if err := helper.Patch(ctx, resourcePolicy); err != nil {
		return "", err
	}

	return resourcePolicyName, nil
}

func (s *RPService) mutateResourcePolicy(resourcePolicy *vmoprv1.VirtualMachineSetResourcePolicy, clusterModuleGroups []string, cluster *clusterv1.Cluster, vSphereCluster *vmwarev1.VSphereCluster, isCreate bool) error {
	// Always ensure the owner reference
	if err := ctrlutil.SetOwnerReference(vSphereCluster, resourcePolicy, s.Client.Scheme()); err != nil {
		return errors.Wrapf(err, "failed to set owner reference for virtualMachineSetResourcePolicy %s for cluster %s", klog.KObj(resourcePolicy), klog.KObj(vSphereCluster))
	}

	// Always ensure the clusterModuleGroups are up-to-date.
	resourcePolicy.Spec.ClusterModuleGroups = clusterModuleGroups

	// On create: Also set resourcePool and folder
	if isCreate {
		resourcePolicy.Spec.Folder = cluster.Name
		resourcePolicy.Spec.ResourcePool = vmoprv1.ResourcePoolSpec{
			Name: cluster.Name,
		}
	}

	return nil
}

func getVirtualMachineSetResourcePolicy(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster) (*vmoprv1.VirtualMachineSetResourcePolicy, error) {
	vmResourcePolicy := &vmoprv1.VirtualMachineSetResourcePolicy{}
	vmResourcePolicyName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}
	if err := ctrlClient.Get(ctx, vmResourcePolicyName, vmResourcePolicy); err != nil {
		return nil, err
	}

	return vmResourcePolicy, nil
}

func getFallbackWorkerClusterModuleGroupName(clusterName string) string {
	return fmt.Sprintf("%s-workers-0", clusterName)
}

func getWorkerAntiAffinityMode(vSphereCluster *vmwarev1.VSphereCluster) vmwarev1.VSphereClusterWorkerAntiAffinityMode {
	if vSphereCluster.Spec.Placement == nil || vSphereCluster.Spec.Placement.WorkerAntiAffinity == nil {
		return vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster
	}

	return vSphereCluster.Spec.Placement.WorkerAntiAffinity.Mode
}

func getTargetClusterModuleGroups(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster, vSphereCluster *vmwarev1.VSphereCluster) ([]string, error) {
	if !feature.Gates.Enabled(feature.WorkerAntiAffinity) {
		// Fallback to old behaviour
		return []string{
			ControlPlaneVMClusterModuleGroupName,
			getFallbackWorkerClusterModuleGroupName(cluster.Name),
		}, nil
	}
	// Always add a cluster module for control plane machines.
	modules := []string{
		ControlPlaneVMClusterModuleGroupName,
	}

	switch mode := getWorkerAntiAffinityMode(vSphereCluster); mode {
	case vmwarev1.VSphereClusterWorkerAntiAffinityModeNone:
		// Only configure a cluster module for control-plane nodes
	case vmwarev1.VSphereClusterWorkerAntiAffinityModeCluster:
		// Add an additional cluster module for workers when using Cluster mode.
		modules = append(modules, ClusterWorkerVMClusterModuleGroupName)
	case vmwarev1.VSphereClusterWorkerAntiAffinityModeMachineDeployment:
		// Add an additional cluster module for each MachineDeployment workers when using MachineDeployment mode.
		machineDeploymentNames, err := getMachineDeploymentNamesForCluster(ctx, ctrlClient, cluster)
		if err != nil {
			return nil, err
		}

		modules = append(modules, machineDeploymentNames...)
	default:
		return nil, errors.Errorf("unknown mode %q configured for WorkerAntiAffinity", mode)
	}

	// Add cluster modules from existing VirtualMachines and deduplicate with the target ones.
	existingModules, err := getVirtualMachineClusterModulesForCluster(ctx, ctrlClient, cluster)
	if err != nil {
		return nil, err
	}
	modules = existingModules.Insert(modules...).UnsortedList()

	// Sort elements to have deterministic output.
	sort.Strings(modules)

	return modules, nil
}

func getVirtualMachineClusterModulesForCluster(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster) (sets.Set[string], error) {
	labels := map[string]string{clusterv1.ClusterNameLabel: cluster.GetName()}
	virtualMachineList := &vmoprv1.VirtualMachineList{}
	if err := ctrlClient.List(
		ctx, virtualMachineList,
		client.InNamespace(cluster.GetNamespace()),
		client.MatchingLabels(labels)); err != nil {
		return nil, errors.Wrapf(err, "failed to list MachineDeployment objects")
	}

	clusterModules := sets.Set[string]{}
	for _, virtualMachine := range virtualMachineList.Items {
		if clusterModule, ok := virtualMachine.Annotations[ClusterModuleNameAnnotationKey]; ok {
			clusterModules = clusterModules.Insert(clusterModule)
		}
	}
	return clusterModules, nil
}

func checkClusterModuleGroup(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster, clusterModuleGroupName string) error {
	resourcePolicy, err := getVirtualMachineSetResourcePolicy(ctx, ctrlClient, cluster)
	if err != nil {
		return err
	}

	for _, cm := range resourcePolicy.Status.ClusterModules {
		if cm.GroupName == clusterModuleGroupName {
			return nil
		}
	}

	return errors.Errorf("VirtualMachineSetResourcePolicy's .status.clusterModules does not yet contain group %q", clusterModuleGroupName)
}
