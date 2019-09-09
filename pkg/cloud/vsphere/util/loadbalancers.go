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
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// GetClusterLoadBalancer gets a cluster's LoadBalancer resources.
func GetClusterLoadBalancer(
	ctx context.Context,
	controllerClient client.Client,
	namespace string, clusterName string) (map[string]*infrav1.LoadBalancer, error) {

	labels := map[string]string{clusterv1.MachineClusterLabelName: clusterName}
	loadBalancerList := &infrav1.LoadBalancerList{}

	if err := controllerClient.List(
		ctx, loadBalancerList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	loadBalancers := map[string]*infrav1.LoadBalancer{}
	for i := range loadBalancerList.Items {
		loadBalancerName := loadBalancerList.Items[i].Name
		loadBalancers[loadBalancerName] = &loadBalancerList.Items[i]
	}

	return loadBalancers, nil
}
