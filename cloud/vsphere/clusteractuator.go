/*
Copyright 2018 The Kubernetes Authors.

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

package vsphere

import (
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/constants"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
)

// ClusterActuator represents the vsphere cluster actuator responsible for maintaining the cluster level objects
type ClusterActuator struct {
	clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface
	lister          v1alpha1.Interface
	eventRecorder   record.EventRecorder
}

// Reconcile will create or update the cluster
func (vc *ClusterActuator) Reconcile(cluster *clusterv1.Cluster) error {
	glog.Infof("Attempting to reconcile cluster %s", cluster.ObjectMeta.Name)

	// The generic workflow would be as follows:
	// 1. If cluster.Status.APIEndpoints is not there, spawn a lb and generate an endpoint
	// for the API endpoint and populate that endpoint in the status
	// 2. If the cluster.Status.APIEndpoints is there, then ensure that the members of the lb
	// match the list of master nodes for this cluster.
	// In the absence of the lb creation, the logic would be to simply take the first master node
	// and use that as the API endpoint for now.
	if len(cluster.Status.APIEndpoints) == 0 {
		err := vc.provisionLoadBalancer(cluster)
		if err != nil {
			glog.Infof("Error could not provision the Load Balancer for the cluster: %s", err)
			return err
		}
		// uncomment the below return statement once we actually add the lb implementation
		// since the provisionLoadBalancer would trigger another Reconcile loop as it updates the endpoints
		//return nil
	}
	// At this stage we are expecting the lb endpoint to be present in the final lb implementation
	err := vc.ensureLoadBalancerMembers(cluster)
	if err != nil {
		glog.Infof("Error setting the Load Balancer members for the cluster: %s", err)
		return err
	}
	return nil
}

func (vc *ClusterActuator) provisionLoadBalancer(cluster *clusterv1.Cluster) error {
	// TODO(ssurana):
	// 1. implement the lb provisioning
	// 2. update the lb public endpoint to the cluster endpoint
	return nil
}

// ensureLoadBalancerMembers would be responsible for keeping the master API endpoints
// synced with the lb members at all times.
func (vc *ClusterActuator) ensureLoadBalancerMembers(cluster *clusterv1.Cluster) error {
	// This is the temporary implementation until we do the proper LB implementation
	err := vc.setMasterNodeIPAsEndpoint(cluster)
	if err != nil {
		glog.Infof("Error registering master node's IP as API Endpoint for the cluster: %s", err)
		return err
	}
	return nil
}

// TODO(ssurana): Remove this method once we have the proper lb implementation
// Temporary implementation: Simply use the first master IP that you can find
func (vc *ClusterActuator) setMasterNodeIPAsEndpoint(cluster *clusterv1.Cluster) error {
	if len(cluster.Status.APIEndpoints) == 0 {
		masters, err := vsphereutils.GetMasterForCluster(cluster, vc.lister)
		if err != nil {
			glog.Infof("Error retrieving master nodes for the cluster: %s", err)
			return err
		}
		for _, master := range masters {
			ip, err := vsphereutils.GetIP(cluster, master)
			if err != nil {
				glog.Infof("Master node [%s] IP not ready yet: %s", master.Name, err)
				// continue the loop to see if there are any other master available that has the
				// IP already populated
				continue
			}
			cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{
				clusterv1.APIEndpoint{
					Host: ip,
					Port: constants.ApiServerPort,
				}}
			_, err = vc.clusterV1alpha1.Clusters(cluster.Namespace).UpdateStatus(cluster)
			if err != nil {
				vc.eventRecorder.Eventf(cluster, corev1.EventTypeWarning, "Failed Update", "Error in updating API Endpoint: %s", err)
				glog.Infof("Error in updating the status: %s", err)
				return err
			}
			vc.eventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Updated", "Updated API Endpoint to %v", ip)
		}
	}
	return nil
}

// Delete will delete any cluster level resources for the cluster.
func (vc *ClusterActuator) Delete(cluster *clusterv1.Cluster) error {
	vc.eventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Deleted", "Deleting cluster %s", cluster.Name)
	glog.Infof("Attempting to cleaning up resources of cluster %s", cluster.ObjectMeta.Name)
	return nil
}

// NewClusterActuator creates the instance for the ClusterActuator
func NewClusterActuator(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder) (*ClusterActuator, error) {
	return &ClusterActuator{
		clusterV1alpha1: clusterV1alpha1,
		lister:          lister,
		eventRecorder:   eventRecorder,
	}, nil
}
