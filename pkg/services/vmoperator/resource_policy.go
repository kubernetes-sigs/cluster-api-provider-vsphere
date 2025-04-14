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
	"strings"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// RPService represents the ability to reconcile a VirtualMachineSetResourcePolicy via vmoperator.
type RPService struct {
	Client client.Client
}

// ReconcileResourcePolicy ensures that a VirtualMachineSetResourcePolicy exists for the cluster
// Returns the name of a policy if it exists, otherwise returns an error.
func (s *RPService) ReconcileResourcePolicy(ctx context.Context, cluster *clusterv1.Cluster, vSphereCluster *vmwarev1.VSphereCluster) (string, error) {
	clusterModuleGroups, err := getTargetClusterModuleGroups(ctx, s.Client, cluster)
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

func getTargetClusterModuleGroups(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster) ([]string, error) {
	if !feature.Gates.Enabled(feature.WorkerAntiAffinity) {
		// Fallback to old behaviour
		return []string{
			ControlPlaneVMClusterModuleGroupName,
			getFallbackWorkerClusterModuleGroupName(cluster.Name),
		}, nil
	}

	// Always add a cluster module for control plane machines.
	modules := sets.New(ControlPlaneVMClusterModuleGroupName)

	// Get all worker related cluster modules by listing all machines and reading from their VSphereMachine objects.
	clusterModules, err := getWorkerVSphereMachineClusterModuleNamesForCluster(ctx, ctrlClient, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cluster module names for workers")
	}

	modules.Insert(clusterModules...)
	modulesList := modules.UnsortedList()
	// Sort elements to have deterministic output.
	sort.Strings(modulesList)

	return modulesList, nil
}

func getWorkerVSphereMachineClusterModuleNamesForCluster(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster) ([]string, error) {
	log := ctrl.LoggerFrom(ctx)
	// Get all worker CAPI machines for the cluster.
	machines, err := collections.GetFilteredMachinesForCluster(ctx, ctrlClient, cluster, func(machine *clusterv1.Machine) bool {
		return !collections.ControlPlaneMachines(cluster.Name)(machine)
	})
	if err != nil {
		return nil, errors.Wrapf(err,
			"failed to get Machines for Cluster %s/%s",
			cluster.Namespace, cluster.Name)
	}

	// Collect all resulting Cluster Module names for the cluster.
	clusterModuleNames := []string{}
	for _, machine := range machines {
		// Note: We have to use := here to create a new variable and not overwrite log & ctx outside the for loop.
		log := log.WithValues("Machine", klog.KObj(machine))
		ctx := ctrl.LoggerInto(ctx, log)

		// Get the vSphereMachine for the CAPI Machine resource.
		vSphereMachine, err := util.GetVSphereMachine(ctx, ctrlClient, machine.Namespace, machine.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get VSphereMachine for Machine %s/%s", machine.Namespace, machine.Name)
		}
		log = log.WithValues("VSphereMachine", klog.KObj(vSphereMachine))
		ctx = ctrl.LoggerInto(ctx, log) //nolint:ineffassign,staticcheck // ensure the logger is up-to-date in ctx, even if we currently don't use ctx below.

		clusterModuleName, err := getClusterModuleName(cluster.Name, machine, vSphereMachine)
		if err != nil {
			return nil, err
		}
		if clusterModuleName != "" {
			clusterModuleNames = append(clusterModuleNames, clusterModuleName)
		}
	}

	return clusterModuleNames, nil
}

func getClusterModuleName(clusterName string, machine *clusterv1.Machine, vSphereMachine *vmwarev1.VSphereMachine) (string, error) {
	// Fallback to cluster-wide module name if nothing is configured.
	if vSphereMachine.Spec.Affinity == nil || vSphereMachine.Spec.Affinity.MachineDeploymentMachineAntiAffinity == nil || len(vSphereMachine.Spec.Affinity.MachineDeploymentMachineAntiAffinity.PreferredDuringSchedulingPreferredDuringExecution) == 0 {
		// Fallback to old name if feature-gate is disabled.
		if !feature.Gates.Enabled(feature.WorkerAntiAffinity) {
			return getFallbackWorkerClusterModuleGroupName(clusterName), nil
		}
		return ClusterWorkerVMClusterModuleGroupName, nil
	}

	if len(vSphereMachine.Spec.Affinity.MachineDeploymentMachineAntiAffinity.PreferredDuringSchedulingPreferredDuringExecution) > 1 {
		// This should never happen because we block this during validation.
		return "", fmt.Errorf("VSphereMachine %s has more then one item set at preferredDuringSchedulingPreferredDuringExecution", klog.KObj(vSphereMachine))
	}

	term := vSphereMachine.Spec.Affinity.MachineDeploymentMachineAntiAffinity.PreferredDuringSchedulingPreferredDuringExecution[0]

	// Return no name in case of no matchLabelKeys.
	// This leads to disable cluster-modules for this machine.
	if len(term.MatchLabelKeys) == 0 {
		return "", nil
	}

	// Return cluster-wide module name if only the cluster name label is set
	if len(term.MatchLabelKeys) == 1 && term.MatchLabelKeys[0] == clusterv1.ClusterNameLabel {
		// Fallback to old name if feature-gate is disabled.
		if !feature.Gates.Enabled(feature.WorkerAntiAffinity) {
			return getFallbackWorkerClusterModuleGroupName(clusterName), nil
		}
		return ClusterWorkerVMClusterModuleGroupName, nil
	}

	// This requires to have enabled the WorkerAntiAffinity feature-gate.
	if !feature.Gates.Enabled(feature.WorkerAntiAffinity) {
		return "", fmt.Errorf("VSphereMachine %s has an invalid or more then one matchLabelKey set for MachineDeploymentMachineAntiAffinity, but WorkerAntiAffinity feature gate is disabled", klog.KObj(vSphereMachine))
	}

	values := []string{}
	sort.Strings(term.MatchLabelKeys)
	for _, key := range term.MatchLabelKeys {
		// There's no need to add the cluster name label's value to the cluster module name
		// because they are already cluster scoped.
		if key == clusterv1.ClusterNameLabel {
			continue
		}
		values = append(values, machine.Labels[key])
	}

	return strings.Join(values, "_"), nil
}
