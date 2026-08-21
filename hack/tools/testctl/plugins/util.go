/*
Copyright The Kubernetes Authors.

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

package plugins

import (
	"context"
	"regexp"
	"sort"

	pkgerrors "github.com/pkg/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getClusters(ctx context.Context, c client.Client, nameRegEx string, limit *int32) ([]*clusterv1.Cluster, error) {
	clusterList := &clusterv1.ClusterList{}
	if err := c.List(ctx, clusterList); err != nil {
		return nil, pkgerrors.Wrap(err, "failed to list Clusters")
	}

	var regex *regexp.Regexp
	if nameRegEx != "" {
		var err error
		regex, err = regexp.Compile(nameRegEx)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to parse nameRegex %s", nameRegEx)
		}
	}

	clusters := make([]*clusterv1.Cluster, 0, len(clusterList.Items))
	for _, cluster := range clusterList.Items {
		if regex != nil && !regex.MatchString(cluster.Name) {
			continue
		}

		clusters = append(clusters, &cluster)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return klog.KObj(clusters[i]).String() < klog.KObj(clusters[j]).String()
	})

	if limit != nil && len(clusters) >= int(ptr.Deref(limit, 0)) {
		return clusters[:int(ptr.Deref(limit, 0))], nil
	}
	return clusters, nil
}

func getControlPlane(ctx context.Context, c client.Client, cluster *clusterv1.Cluster) (*controlplanev1.KubeadmControlPlane, error) {
	controlPlane := &controlplanev1.KubeadmControlPlane{}
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	if err := c.Get(ctx, key, controlPlane); err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to get %s %s for Cluster %s", cluster.Spec.ControlPlaneRef.Kind, klog.KObj(controlPlane), klog.KObj(cluster))
	}
	return controlPlane, nil
}

func getControlPlaneMachines(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, controlPlane *controlplanev1.KubeadmControlPlane) ([]*clusterv1.Machine, error) {
	machineList := &clusterv1.MachineList{}

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			clusterv1.MachineControlPlaneLabel: "",
			clusterv1.ClusterNameLabel:         cluster.Name,
		},
	}
	if err := c.List(ctx, machineList, listOptions...); err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to list Machines for KubeadmControlPlane %s", klog.KObj(controlPlane))
	}

	machines := []*clusterv1.Machine{}
	for _, machine := range machineList.Items {
		machines = append(machines, &machine)
	}
	return machines, nil
}

func waitForControlPlaneMachines(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, controlPlane *controlplanev1.KubeadmControlPlane, replicas int32, version string) (done bool, err error, retryErr error) {
	machines, err := getControlPlaneMachines(ctx, c, cluster, controlPlane)
	if err != nil {
		return false, err, nil
	}

	if int32(len(machines)) != replicas {
		return false, nil, pkgerrors.Errorf("waiting for %d KubeadmControlPlane machines to exist, found %d", replicas, len(machines))
	}

	for _, m := range machines {
		if m.Spec.Version != version {
			return false, nil, pkgerrors.Errorf("waiting for %d KubeadmControlPlane machines to have version %s", replicas, version)
		}

		if !conditions.IsTrue(m, clusterv1.MachineAvailableCondition) {
			return false, nil, pkgerrors.Errorf("waiting for %d KubeadmControlPlane machines to have Available condition True", replicas)
		}
	}
	return true, nil, nil
}

func getMachineDeployments(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, topologyNameRegEx string, limit *int32) ([]*clusterv1.MachineDeployment, error) {
	machineDeploymentList := &clusterv1.MachineDeploymentList{}

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
	}
	if err := c.List(ctx, machineDeploymentList, listOptions...); err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to list MachineDeployments for Cluster %s", klog.KObj(cluster))
	}

	var regex *regexp.Regexp
	if topologyNameRegEx != "" {
		var err error
		regex, err = regexp.Compile(topologyNameRegEx)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to parse nameRegex %s", topologyNameRegEx)
		}
	}

	machineDeployments := make([]*clusterv1.MachineDeployment, 0, len(machineDeploymentList.Items))
	for _, machineDeployment := range machineDeploymentList.Items {
		if topologyName, ok := machineDeployment.Labels[clusterv1.ClusterTopologyMachineDeploymentNameLabel]; ok && regex != nil {
			if !regex.MatchString(topologyName) {
				continue
			}
		}

		machineDeployments = append(machineDeployments, &machineDeployment)
	}

	sort.Slice(machineDeployments, func(i, j int) bool {
		return klog.KObj(machineDeployments[i]).String() < klog.KObj(machineDeployments[j]).String()
	})

	if limit != nil && len(machineDeployments) >= int(ptr.Deref(limit, 0)) {
		return machineDeployments[:int(ptr.Deref(limit, 0))], nil
	}
	return machineDeployments, nil
}

func getMachineDeployment(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, mdTopologyName string) (*clusterv1.MachineDeployment, error) {
	machineDeploymentList := &clusterv1.MachineDeploymentList{}
	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel:                          cluster.Name,
			clusterv1.ClusterTopologyMachineDeploymentNameLabel: mdTopologyName,
		},
	}
	if err := c.List(ctx, machineDeploymentList, listOptions...); err != nil {
		return nil, err
	}
	if len(machineDeploymentList.Items) == 1 {
		return &machineDeploymentList.Items[0], nil
	}
	if len(machineDeploymentList.Items) > 1 {
		return nil, pkgerrors.Errorf("there is more than one MachineDeployment with the %s=%s label", clusterv1.ClusterTopologyMachineDeploymentNameLabel, mdTopologyName)
	}
	return nil, nil
}

func getMachineDeploymentMachines(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, machineDeployment *clusterv1.MachineDeployment) ([]*clusterv1.Machine, error) {
	machineList := &clusterv1.MachineList{}

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel: cluster.Name,
		},
		client.MatchingLabels(machineDeployment.Spec.Selector.MatchLabels),
	}
	if err := c.List(ctx, machineList, listOptions...); err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to list Machines for MachineDeployment %s", klog.KObj(machineDeployment))
	}

	machines := []*clusterv1.Machine{}
	for _, machine := range machineList.Items {
		machines = append(machines, &machine)
	}
	return machines, nil
}

func waitForMachineDeploymentMachines(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, machineDeployment *clusterv1.MachineDeployment, replicas int32, version string) (done bool, err error, retryErr error) {
	machines, err := getMachineDeploymentMachines(ctx, c, cluster, machineDeployment)
	if err != nil {
		return false, err, nil
	}

	if int32(len(machines)) != replicas {
		return false, nil, pkgerrors.Errorf("waiting for %d MachineDeployment machines to exist, found %d", replicas, len(machines))
	}

	for _, m := range machines {
		if m.Spec.Version != version {
			return false, nil, pkgerrors.Errorf("waiting for %d KubeadmControlPlane machines to have version %s", replicas, version)
		}

		if !conditions.IsTrue(m, clusterv1.MachineAvailableCondition) {
			return false, nil, pkgerrors.Errorf("waiting for %d MachineDeployment machines to have Available condition True", replicas)
		}
	}
	return true, nil, nil
}
