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
	"sort"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
)

// RPService represents the ability to reconcile a VirtualMachineSetResourcePolicy via vmoperator.
type RPService struct {
	Client client.Client
}

// ReconcileResourcePolicy ensures that a VirtualMachineSetResourcePolicy exists for the cluster
// Returns the name of a policy if it exists, otherwise returns an error.
func (s *RPService) ReconcileResourcePolicy(ctx context.Context, clusterCtx *vmware.ClusterContext) (string, error) {
	clusterModuleGroups, err := getClusterModuleGroups(ctx, s.Client, clusterCtx.Cluster)
	if err != nil {
		return "", err
	}

	resourcePolicy, err := getVirtualMachineSetResourcePolicy(ctx, s.Client, clusterCtx.Cluster)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", errors.Errorf("unexpected error in getting the Resource policy: %+v", err)
		}
		resourcePolicy, err = s.createVirtualMachineSetResourcePolicy(ctx, clusterCtx, clusterModuleGroups)
		if err != nil {
			return "", errors.Errorf("failed to create Resource Policy: %+v", err)
		}
		return resourcePolicy.Name, nil
	}

	// Ensure .spec.clusterModuleGroups is up to date.
	helper, err := patch.NewHelper(resourcePolicy, s.Client)
	if err != nil {
		return "", err
	}
	resourcePolicy.Spec.ClusterModuleGroups = clusterModuleGroups
	if err := helper.Patch(ctx, resourcePolicy); err != nil {
		return "", err
	}

	return resourcePolicy.Name, nil
}

func (s *RPService) newVirtualMachineSetResourcePolicy(clusterCtx *vmware.ClusterContext) *vmoprv1.VirtualMachineSetResourcePolicy {
	return &vmoprv1.VirtualMachineSetResourcePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterCtx.Cluster.Namespace,
			Name:      clusterCtx.Cluster.Name,
		},
	}
}

func getVirtualMachineSetResourcePolicy(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster) (*vmoprv1.VirtualMachineSetResourcePolicy, error) {
	vmResourcePolicy := &vmoprv1.VirtualMachineSetResourcePolicy{}
	vmResourcePolicyName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}
	err := ctrlClient.Get(ctx, vmResourcePolicyName, vmResourcePolicy)
	return vmResourcePolicy, err
}

func (s *RPService) createVirtualMachineSetResourcePolicy(ctx context.Context, clusterCtx *vmware.ClusterContext, clusterModuleGroups []string) (*vmoprv1.VirtualMachineSetResourcePolicy, error) {
	vmResourcePolicy := s.newVirtualMachineSetResourcePolicy(clusterCtx)

	_, err := ctrlutil.CreateOrPatch(ctx, s.Client, vmResourcePolicy, func() error {
		vmResourcePolicy.Spec = vmoprv1.VirtualMachineSetResourcePolicySpec{
			ResourcePool: vmoprv1.ResourcePoolSpec{
				Name: clusterCtx.Cluster.Name,
			},
			Folder:              clusterCtx.Cluster.Name,
			ClusterModuleGroups: clusterModuleGroups,
		}
		// Ensure that the VirtualMachineSetResourcePolicy is owned by the VSphereCluster
		if err := ctrlutil.SetOwnerReference(
			clusterCtx.VSphereCluster,
			vmResourcePolicy,
			s.Client.Scheme(),
		); err != nil {
			return errors.Wrapf(
				err,
				"error setting %s/%s as owner of %s/%s",
				clusterCtx.VSphereCluster.Namespace,
				clusterCtx.VSphereCluster.Name,
				vmResourcePolicy.Namespace,
				vmResourcePolicy.Name,
			)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return vmResourcePolicy, nil
}

func getClusterModuleGroups(ctx context.Context, ctrlClient client.Client, cluster *clusterv1.Cluster) ([]string, error) {
	machineDeploymentNames, err := getMachineDeploymentNamesForCluster(ctx, ctrlClient, cluster)
	if err != nil {
		return nil, err
	}

	clusterModuleGroups := append([]string{ControlPlaneVMClusterModuleGroupName}, machineDeploymentNames...)

	// sort elements to have deterministic output.
	sort.Strings(clusterModuleGroups)

	return clusterModuleGroups, nil
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

	return errors.Errorf("VirtualMachineSetResourcePolicy's .status.clusterModules does not yet contain %s", clusterModuleGroupName)
}
