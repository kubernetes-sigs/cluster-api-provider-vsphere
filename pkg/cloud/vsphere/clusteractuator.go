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
	"net"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/patch"

	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
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
func (a *ClusterActuator) Reconcile(cluster *clusterv1.Cluster) (result error) {
	klog.V(4).Infof("reconciling cluster %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name)

	clusterCopy := cluster.DeepCopy()

	defer func() {
		if err := a.patchCluster(cluster, clusterCopy); err != nil {
			if result == nil {
				result = err
			} else {
				result = errors.Wrap(result, err.Error())
			}
		}
	}()

	// Ensure the PKI config is present or generated and then set the updated
	// clusterConfig back onto the cluster.
	if err := certificates.ReconcileCertificates(cluster); err != nil {
		return errors.Wrapf(err,
			"unable to reconcile certs while reconciling cluster %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
	}

	return nil
}

// Delete will delete any cluster level resources for the cluster.
func (a *ClusterActuator) Delete(cluster *clusterv1.Cluster) error {
	a.eventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Deleted", "Deleting cluster %s", cluster.Name)
	klog.Infof("Attempting to cleaning up resources of cluster %s", cluster.ObjectMeta.Name)
	return nil
}

func (a *ClusterActuator) patchCluster(cluster, clusterCopy *clusterv1.Cluster) error {

	clusterClient := a.clusterV1alpha1.Clusters(cluster.Namespace)

	clusterConfig, err := vsphereconfigv1.ClusterConfigFromCluster(cluster)
	if err != nil {
		return errors.Wrapf(err,
			"unable to get cluster provider spec for cluster while patching cluster %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
	}

	clusterStatus, err := vsphereconfigv1.ClusterStatusFromCluster(cluster)
	if err != nil {
		return errors.Wrapf(err,
			"unable to get cluster provider status for cluster while patching cluster %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
	}

	ext, err := vsphereconfigv1.EncodeClusterSpec(clusterConfig)
	if err != nil {
		return errors.Wrap(err, "failed encoding cluster spec")
	}
	newStatus, err := vsphereconfigv1.EncodeClusterStatus(clusterStatus)
	if err != nil {
		return errors.Wrap(err, "failed encoding cluster status")
	}
	ext.Object = nil
	newStatus.Object = nil

	cluster.Spec.ProviderSpec.Value = ext

	// Build a patch and marshal that patch to something the client will understand.
	p, err := patch.NewJSONPatch(clusterCopy, cluster)
	if err != nil {
		return errors.Wrap(err, "failed to create new JSONPatch")
	}

	klog.Infof(
		"generated json patch for cluster %s=%s %s=%s %s=%v",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name,
		"json-patch", p)

	// Do not update Machine if nothing has changed
	if len(p) != 0 {

		pb, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return errors.Wrap(err, "failed to json marshal patch")
		}

		klog.V(1).Infof(
			"patching cluster %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)

		result, err := clusterClient.Patch(cluster.Name, types.JSONPatchType, pb)
		if err != nil {
			a.eventRecorder.Eventf(
				cluster, corev1.EventTypeWarning,
				"UpdateFailure",
				"failed to update cluster config %s=%s %s=%s %s=%v",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Name,
				"error", err)
			return errors.Wrap(err, "failed to patch cluster")
		}

		a.eventRecorder.Eventf(
			cluster, corev1.EventTypeNormal,
			"UpdateSuccess",
			"updated cluster config %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)

		// Keep the resource version updated so the status update can succeed
		cluster.ResourceVersion = result.ResourceVersion
	}

	// If the cluster is online then update the cluster's APIEndpoints
	// to include the control plane endpoint.
	if ok, controlPlaneEndpoint, _ := vsphereutils.GetControlPlaneStatus(cluster, a.lister); ok {
		host, szPort, err := net.SplitHostPort(controlPlaneEndpoint)
		if err != nil {
			return errors.Wrapf(err,
				"unable to get host/port for control plane endpoint %s=%s %s=%s %s=%s",
				"control-plane-endpoint", controlPlaneEndpoint,
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Name)
		}
		port, err := strconv.Atoi(szPort)
		if err != nil {
			return errors.Wrapf(err,
				"unable to get parse port for control plane endpoint %s=%s %s=%s %s=%s %s=%s",
				"host", host,
				"port", szPort,
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Name)
		}
		if len(cluster.Status.APIEndpoints) == 0 || (cluster.Status.APIEndpoints[0].Host != host && cluster.Status.APIEndpoints[0].Port != port) {
			cluster.Status.APIEndpoints = []clusterv1.APIEndpoint{
				clusterv1.APIEndpoint{
					Host: host,
					Port: port,
				},
			}
		}
	}
	cluster.Status.ProviderStatus = newStatus

	if !reflect.DeepEqual(cluster.Status, clusterCopy.Status) {
		klog.V(1).Infof(
			"updating cluster status %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
		if _, err := clusterClient.UpdateStatus(cluster); err != nil {
			a.eventRecorder.Eventf(
				cluster, corev1.EventTypeWarning,
				"UpdateFailure",
				"failed to update cluster status %s=%s %s=%s %s=%v",
				"cluster-namespace", cluster.Namespace,
				"cluster-name", cluster.Name,
				"error", err)
			return errors.Wrap(err, "failed to update cluster status")
		}
	}

	a.eventRecorder.Eventf(
		cluster, corev1.EventTypeNormal,
		"UpdateSuccess",
		"updated cluster status %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name)

	return nil
}
