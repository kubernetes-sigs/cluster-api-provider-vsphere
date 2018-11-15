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
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	clustererror "sigs.k8s.io/cluster-api/pkg/controller/error"
)

// ClusterActuator represents the vsphere cluster actuator responsible for maintaining the cluster level objects
type ClusterActuator struct {
	clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface
	lister          v1alpha1.Interface
	eventRecorder   record.EventRecorder
	k8sClient       kubernetes.Interface
}

// NewClusterActuator creates the instance for the ClusterActuator
func NewClusterActuator(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, k8sClient kubernetes.Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder) (*ClusterActuator, error) {
	return &ClusterActuator{
		clusterV1alpha1: clusterV1alpha1,
		lister:          lister,
		eventRecorder:   eventRecorder,
		k8sClient:       k8sClient,
	}, nil
}

// Reconcile will create or update the cluster
func (ca *ClusterActuator) Reconcile(cluster *clusterv1.Cluster) error {
	glog.V(4).Infof("Attempting to reconcile cluster %s", cluster.ObjectMeta.Name)

	// The generic workflow would be as follows:
	// 1. If cluster.Status.APIEndpoints is not there, spawn a lb and generate an endpoint
	// for the API endpoint and populate that endpoint in the status
	// 2. If the cluster.Status.APIEndpoints is there, then ensure that the members of the lb
	// match the list of master nodes for this cluster.
	// In the absence of the lb creation, the logic would be to simply take the first master node
	// and use that as the API endpoint for now.
	if len(cluster.Status.APIEndpoints) == 0 {
		err := ca.provisionLoadBalancer(cluster)
		if err != nil {
			glog.Infof("Error could not provision the Load Balancer for the cluster: %s", err)
			return err
		}
		// uncomment the below return statement once we actually add the lb implementation
		// since the provisionLoadBalancer would trigger another Reconcile loop as it updates the endpoints
		//return nil
	}
	// At this stage we are expecting the lb endpoint to be present in the final lb implementation
	err := ca.ensureLoadBalancerMembers(cluster)
	if err != nil {
		glog.Infof("Error setting the Load Balancer members for the cluster: %s", err)
		return err
	}
	// Check if the target kubernetes is ready or not, and update the ProviderStatus if change is detected
	err = ca.updateK8sAPIStatus(cluster)
	if err != nil {
		return err
	}
	return nil
}

func (ca *ClusterActuator) updateK8sAPIStatus(cluster *clusterv1.Cluster) error {
	currentClusterAPIStatus, err := ca.getClusterAPIStatus(cluster)
	if err != nil {
		glog.V(4).Infof("ClusterActuator failed to get cluster status: %s", err.Error())
		return err
	}
	return ca.updateClusterAPIStatus(cluster, currentClusterAPIStatus)
}

// fetchKubeConfig returns the cached copy of the Kubeconfig in the secrets for the target cluster
// In case the secret does not exist, then it fetches from the target master node and caches it for
func (ca *ClusterActuator) fetchKubeConfig(cluster *clusterv1.Cluster, masters []*clusterv1.Machine) (string, error) {
	var kubeconfig string
	glog.V(4).Infof("attempting to fetch kubeconfig")
	secret, err := ca.k8sClient.Core().Secrets(cluster.Namespace).Get(fmt.Sprintf(constants.KubeConfigSecretName, cluster.UID), metav1.GetOptions{})
	if err != nil {
		glog.V(4).Info("could not pull secrets for kubeconfig")
		// TODO: Check for the proper err type for *not present* case. rather than all other cases
		// Fetch the kubeconfig and create the secret saving it
		// Currently we support only a single master thus the below assumption
		// Once we start supporting multiple masters, the kubeconfig needs to
		// be generated differently, with the URL from the LB endpoint
		master := masters[0]
		kubeconfig, err = vsphereutils.GetKubeConfig(cluster, master)
		if err != nil || kubeconfig == "" {
			glog.Infof("[cluster-actuator] error retrieving kubeconfig for target cluster, will requeue")
			return "", &clustererror.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
		}
		configmap := make(map[string]string)
		configmap[constants.KubeConfigSecretData] = kubeconfig
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf(constants.KubeConfigSecretName, cluster.UID),
			},
			StringData: configmap,
		}
		secret, err = ca.k8sClient.Core().Secrets(cluster.Namespace).Create(secret)
		if err != nil {
			glog.Warningf("Could not create the secret for the saving kubeconfig: err [%s]", err.Error())
		}
	} else {
		glog.V(4).Info("found kubeconfig in secrets")
		kubeconfig = string(secret.Data[constants.KubeConfigSecretData])
	}
	return kubeconfig, nil
}

func (ca *ClusterActuator) getClusterAPIStatus(cluster *clusterv1.Cluster) (vsphereconfigv1.APIStatus, error) {
	masters, err := vsphereutils.GetMasterForCluster(cluster, ca.lister)
	if err != nil {
		glog.Infof("Error retrieving master nodes for the cluster: %s", err)
		return vsphereconfigv1.ApiNotReady, err
	}
	if len(masters) == 0 {
		glog.Infof("No masters for the cluster [%s] present", cluster.Name)
		return vsphereconfigv1.ApiNotReady, nil
	}
	kubeconfig, err := ca.fetchKubeConfig(cluster, masters)
	if err != nil {
		return vsphereconfigv1.ApiNotReady, err
	}
	kconfigFile, err := vsphereutils.CreateTempFile(kubeconfig)
	if err != nil {
		return vsphereconfigv1.ApiNotReady, err
	}
	clientConfig, err := clientcmd.BuildConfigFromFlags("", kconfigFile)
	if err != nil {
		glog.Infof("[cluster-actuator] error creating client config for target cluster [%s], will requeue", err.Error())
		return vsphereconfigv1.ApiNotReady, &clustererror.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
	}
	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		glog.Infof("[cluster-actuator] error creating clientset for target cluster [%s], will requeue", err.Error())
		return vsphereconfigv1.ApiNotReady, &clustererror.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
	}
	_, err = clientSet.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		glog.Infof("[cluster-actuator] target cluster API not yet ready [%s], will requeue", err.Error())
		return vsphereconfigv1.ApiNotReady, &clustererror.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds}
	}
	return vsphereconfigv1.ApiReady, nil
}

func (ca *ClusterActuator) updateClusterAPIStatus(cluster *clusterv1.Cluster, newStatus vsphereconfigv1.APIStatus) error {
	oldProviderStatus, err := vsphereutils.GetClusterProviderStatus(cluster)
	if err != nil {
		return err
	}
	if oldProviderStatus != nil && oldProviderStatus.APIStatus == newStatus {
		// Nothing to update
		return nil
	}
	newProviderStatus := &vsphereconfigv1.VsphereClusterProviderStatus{}
	// create a copy of the old status so that any other fields except the ones we want to change can be retained
	if oldProviderStatus != nil {
		newProviderStatus = oldProviderStatus.DeepCopy()
	}
	newProviderStatus.APIStatus = newStatus
	newProviderStatus.LastUpdated = time.Now().UTC().String()
	out, err := json.Marshal(newProviderStatus)
	ncluster := cluster.DeepCopy()
	ncluster.Status.ProviderStatus = &runtime.RawExtension{Raw: out}

	_, err = ca.clusterV1alpha1.Clusters(ncluster.Namespace).UpdateStatus(ncluster)
	if err != nil {
		glog.V(4).Infof("ClusterActuator failed to update the cluster status: %s", err.Error())
		return err
	}
	return nil
}

func (ca *ClusterActuator) provisionLoadBalancer(cluster *clusterv1.Cluster) error {
	// TODO(ssurana):
	// 1. implement the lb provisioning
	// 2. update the lb public endpoint to the cluster endpoint
	return nil
}

// ensureLoadBalancerMembers would be responsible for keeping the master API endpoints
// synced with the lb members at all times.
func (ca *ClusterActuator) ensureLoadBalancerMembers(cluster *clusterv1.Cluster) error {
	// This is the temporary implementation until we do the proper LB implementation
	err := ca.setMasterNodeIPAsEndpoint(cluster)
	if err != nil {
		glog.Infof("Error registering master node's IP as API Endpoint for the cluster: %s", err)
		return err
	}
	return nil
}

// TODO(ssurana): Remove this method once we have the proper lb implementation
// Temporary implementation: Simply use the first master IP that you can find
func (ca *ClusterActuator) setMasterNodeIPAsEndpoint(cluster *clusterv1.Cluster) error {
	ncluster := cluster.DeepCopy()
	if len(ncluster.Status.APIEndpoints) == 0 {
		masters, err := vsphereutils.GetMasterForCluster(ncluster, ca.lister)
		if err != nil {
			glog.Infof("Error retrieving master nodes for the cluster: %s", err)
			return err
		}
		for _, master := range masters {
			ip, err := vsphereutils.GetIP(ncluster, master)
			if err != nil {
				glog.Infof("Master node [%s] IP not ready yet: %s", master.Name, err)
				// continue the loop to see if there are any other master available that has the
				// IP already populated
				continue
			}
			ncluster.Status.APIEndpoints = []clusterv1.APIEndpoint{
				clusterv1.APIEndpoint{
					Host: ip,
					Port: constants.ApiServerPort,
				}}
			_, err = ca.clusterV1alpha1.Clusters(ncluster.Namespace).UpdateStatus(ncluster)
			if err != nil {
				ca.eventRecorder.Eventf(ncluster, corev1.EventTypeWarning, "Failed Update", "Error in updating API Endpoint: %s", err)
				glog.Infof("Error in updating the status: %s", err)
				return err
			}
			ca.eventRecorder.Eventf(ncluster, corev1.EventTypeNormal, "Updated", "Updated API Endpoint to %v", ip)
		}
	}
	return nil
}

// Delete will delete any cluster level resources for the cluster.
func (ca *ClusterActuator) Delete(cluster *clusterv1.Cluster) error {
	ca.eventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Deleted", "Deleting cluster %s", cluster.Name)
	glog.Infof("Attempting to cleaning up resources of cluster %s", cluster.ObjectMeta.Name)
	return nil
}
