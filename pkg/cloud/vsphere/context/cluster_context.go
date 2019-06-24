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

package context

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	clusterUtilv1 "sigs.k8s.io/cluster-api/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/patch"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
)

// ClusterContextParams are the parameters needed to create a ClusterContext.
type ClusterContextParams struct {
	Context    context.Context
	Cluster    *clusterv1.Cluster
	Client     client.ClusterV1alpha1Interface
	CoreClient corev1.CoreV1Interface
	Logger     logr.Logger
}

// ClusterContext is a Go context used with a CAPI cluster.
type ClusterContext struct {
	context.Context
	Cluster       *clusterv1.Cluster
	ClusterCopy   *clusterv1.Cluster
	ClusterClient client.ClusterInterface
	ClusterConfig *v1alpha1.VsphereClusterProviderConfig
	ClusterStatus *v1alpha1.VsphereClusterProviderStatus
	Logger        logr.Logger
	client        client.ClusterV1alpha1Interface
	machineClient client.MachineInterface
	user          string
	pass          string
}

// NewClusterContext returns a new ClusterContext.
func NewClusterContext(params *ClusterContextParams) (*ClusterContext, error) {

	parentContext := params.Context
	if parentContext == nil {
		parentContext = context.Background()
	}

	var clusterClient client.ClusterInterface
	var machineClient client.MachineInterface
	if params.Client != nil {
		clusterClient = params.Client.Clusters(params.Cluster.Namespace)
		machineClient = params.Client.Machines(params.Cluster.Namespace)
	}

	clusterConfig, err := v1alpha1.ClusterConfigFromCluster(params.Cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load cluster provider config")
	}

	clusterStatus, err := v1alpha1.ClusterStatusFromCluster(params.Cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load cluster provider status")
	}

	logr := params.Logger
	if logr == nil {
		logr = klogr.New().WithName("default-logger")
	}
	logr = logr.WithName(params.Cluster.APIVersion).WithName(params.Cluster.Namespace).WithName(params.Cluster.Name)

	user := clusterConfig.VsphereUser
	pass := clusterConfig.VspherePassword
	if secretName := clusterConfig.VsphereCredentialSecret; secretName != "" {
		if params.CoreClient == nil {
			return nil, errors.Errorf("credential secret %q specified without core client", secretName)
		}
		logr.V(4).Info("fetching vsphere credentials", "secret-name", secretName)
		secret, err := params.CoreClient.Secrets(params.Cluster.Namespace).Get(secretName, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "error reading secret %q for cluster %s/%s", secretName, params.Cluster.Namespace, params.Cluster.Name)
		}
		userBuf, userOk := secret.Data[constants.VsphereUserKey]
		passBuf, passOk := secret.Data[constants.VspherePasswordKey]
		if !userOk || !passOk {
			return nil, errors.Wrapf(err, "improperly formatted secret %q for cluster %s/%s", secretName, params.Cluster.Namespace, params.Cluster.Name)
		}
		user, pass = string(userBuf), string(passBuf)
		logr.V(2).Info("found vSphere credentials")
	}

	return &ClusterContext{
		Context:       parentContext,
		Cluster:       params.Cluster,
		ClusterCopy:   params.Cluster.DeepCopy(),
		ClusterClient: clusterClient,
		ClusterConfig: clusterConfig,
		ClusterStatus: clusterStatus,
		Logger:        logr,
		client:        params.Client,
		machineClient: machineClient,
		user:          user,
		pass:          pass,
	}, nil
}

// NewClusterLoggerContext creates a new ClusterContext with the given logger context.
func NewClusterLoggerContext(parentContext *ClusterContext, loggerContext string) *ClusterContext {
	ctx := &ClusterContext{
		Context:       parentContext.Context,
		Cluster:       parentContext.Cluster,
		ClusterCopy:   parentContext.ClusterCopy,
		ClusterClient: parentContext.ClusterClient,
		ClusterConfig: parentContext.ClusterConfig,
		ClusterStatus: parentContext.ClusterStatus,
		client:        parentContext.client,
		machineClient: parentContext.machineClient,
		user:          parentContext.user,
		pass:          parentContext.pass,
	}
	ctx.Logger = parentContext.Logger.WithName(loggerContext)
	return ctx
}

// Strings returns ClusterNamespace/ClusterName
func (c *ClusterContext) String() string {
	return fmt.Sprintf("%s/%s", c.Cluster.Namespace, c.Cluster.Name)
}

// GetObject returns the Cluster object.
func (c *ClusterContext) GetObject() runtime.Object {
	return c.Cluster
}

// GetLogger returns the Logger.
func (c *ClusterContext) GetLogger() logr.Logger {
	return c.Logger
}

// ClusterName returns the name of the cluster.
func (c *ClusterContext) ClusterName() string {
	return c.Cluster.Name
}

// ClusterProviderConfig returns the cluster provider config.
func (c *ClusterContext) ClusterProviderConfig() *v1alpha1.VsphereClusterProviderConfig {
	return c.ClusterConfig
}

// User returns the username used to access the vSphere endpoint.
func (c *ClusterContext) User() string {
	return c.user
}

// Pass returns the password used to access the vSphere endpoint.
func (c *ClusterContext) Pass() string {
	return c.pass
}

// CanLogin returns a flag indicating whether the cluster config has
// enough information to login to the vSphere endpoint.
func (c *ClusterContext) CanLogin() bool {
	return c.ClusterConfig.VsphereServer != "" && c.user != ""
}

// GetMachineClient returns a new Machine client for this cluster.
func (c *ClusterContext) GetMachineClient() client.MachineInterface {
	if c.client != nil {
		return c.client.Machines(c.Cluster.Namespace)
	}
	return nil
}

// GetMachines gets the machines in the cluster.
func (c *ClusterContext) GetMachines() ([]*clusterv1.Machine, error) {
	if c.machineClient == nil {
		return nil, errors.New("machineClient is nil")
	}
	labelSet := labels.Set(map[string]string{
		clusterv1.MachineClusterLabelName: c.Cluster.Name,
	})
	list, err := c.machineClient.List(metav1.ListOptions{LabelSelector: labelSet.AsSelector().String()})
	if err != nil {
		return nil, err
	}
	machines := make([]*clusterv1.Machine, len(list.Items))
	for i := range list.Items {
		machines[i] = &list.Items[i]
	}
	return machines, nil
}

// GetControlPlaneMachines returns the control plane machines for the cluster.
func (c *ClusterContext) GetControlPlaneMachines() ([]*clusterv1.Machine, error) {
	machines, err := c.GetMachines()
	if err != nil {
		return nil, err
	}
	return clusterUtilv1.GetControlPlaneMachines(machines), nil
}

// byMachineCreatedTimestamp implements sort.Interface for []clusterv1.Machine
// based on the machine's creation timestamp.
type byMachineCreatedTimestamp []*clusterv1.Machine

func (a byMachineCreatedTimestamp) Len() int      { return len(a) }
func (a byMachineCreatedTimestamp) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byMachineCreatedTimestamp) Less(i, j int) bool {
	return a[i].CreationTimestamp.Before(&a[j].CreationTimestamp)
}

// FirstControlPlaneMachine returns the first control plane machine according
// to the machines' CreationTimestamp property.
func (c *ClusterContext) FirstControlPlaneMachine() (*clusterv1.Machine, error) {
	controlPlaneMachines, err := c.GetControlPlaneMachines()
	if err != nil {
		return nil, errors.Wrap(err, "getting getting control plane machines")
	}
	if len(controlPlaneMachines) == 0 {
		return nil, nil
	}

	// Sort the control plane machines so the first one created is always the
	// one used to provide the address for the control plane endpoint.
	sortedControlPlaneMachines := byMachineCreatedTimestamp(controlPlaneMachines)
	sort.Sort(sortedControlPlaneMachines)

	return sortedControlPlaneMachines[0], nil
}

// ControlPlaneEndpoint returns the control plane endpoint for the cluster.
// If a control plane endpoint was specified in the cluster configuration, then
// that value will be returned.
// Otherwise this function will return the endpoint of the first control plane
// node in the cluster that reports an IP address.
// If no control plane nodes have reported an IP address then this function
// returns an error.
func (c *ClusterContext) ControlPlaneEndpoint() (string, error) {
	if len(c.Cluster.Status.APIEndpoints) > 0 {
		controlPlaneEndpoint := net.JoinHostPort(c.Cluster.Status.APIEndpoints[0].Host, strconv.Itoa(c.Cluster.Status.APIEndpoints[0].Port))
		c.Logger.V(2).Info("got control plane endpoint from cluster APIEndpoints", "control-plane-endpoint", controlPlaneEndpoint)
		return controlPlaneEndpoint, nil
	}

	if controlPlaneEndpoint := c.ClusterConfig.ClusterConfiguration.ControlPlaneEndpoint; controlPlaneEndpoint != "" {
		c.Logger.V(2).Info("got control plane endpoint from cluster config", "control-plane-endpoint", controlPlaneEndpoint)
		return controlPlaneEndpoint, nil
	}

	machine, err := c.FirstControlPlaneMachine()
	if err != nil {
		return "", errors.Wrap(err, "error getting first control plane machine while searching for control plane endpoint")
	}

	if machine == nil {
		return "", errors.New("cluster does not yet have a control plane machine")
	}

	machineCtx, err := NewMachineContextFromClusterContext(c, machine)
	if err != nil {
		return "", errors.Wrap(err, "error creating machine context while searching for control plane endpoint")
	}

	if ipAddr := machineCtx.IPAddr(); ipAddr != "" {
		controlPlaneEndpoint := net.JoinHostPort(ipAddr, strconv.Itoa(int(machineCtx.BindPort())))
		machineCtx.Logger.V(2).Info("got control plane endpoint from machine", "control-plane-endpoint", controlPlaneEndpoint)
		return controlPlaneEndpoint, nil
	}

	return "", errors.New("unable to get control plane endpoint")
}

// Patch updates the cluster on the API server.
func (c *ClusterContext) Patch() error {
	// Ensure the provider spec is encoded.
	newProviderSpec, err := v1alpha1.EncodeClusterSpec(c.ClusterConfig)
	if err != nil {
		return errors.Wrapf(err, "failed encoding cluster spec for cluster %q", c)
	}
	c.Cluster.Spec.ProviderSpec.Value = newProviderSpec

	// Make sure the status isn't part of the JSON patch.
	newStatus := c.Cluster.Status.DeepCopy()
	c.Cluster.Status = clusterv1.ClusterStatus{}
	c.ClusterCopy.Status.DeepCopyInto(&c.Cluster.Status)

	// Build and marshal a patch for the cluster object, minus the status.
	p, err := patch.NewJSONPatch(c.ClusterCopy, c.Cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to create new JSONPatch for cluster %q", c)
	}

	// Do not update Cluster if nothing has changed
	if len(p) != 0 {
		pb, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "failed to json marshal patch for cluster %q", c)
		}

		c.Logger.V(1).Info("patching cluster")
		c.Logger.V(6).Info("generated json patch for cluster", "json-patch", string(pb))

		result, err := c.ClusterClient.Patch(c.Cluster.Name, types.JSONPatchType, pb)
		if err != nil {
			record.Warnf(c.Cluster, updateFailure, "failed to update cluster config %q: %v", c, err)
			return errors.Wrapf(err, "failed to patch cluster %q", c)
		}

		record.Eventf(c.Cluster, updateSuccess, "updated cluster config %q", c)

		// Keep the resource version updated so the status update can succeed
		c.Cluster.ResourceVersion = result.ResourceVersion
	}

	// Put the status back.
	c.Cluster.Status = clusterv1.ClusterStatus{}
	newStatus.DeepCopyInto(&c.Cluster.Status)

	if !reflect.DeepEqual(c.Cluster.Status, c.ClusterCopy.Status) {
		c.Logger.V(1).Info("updating cluster status")
		if _, err := c.ClusterClient.UpdateStatus(c.Cluster); err != nil {
			record.Warnf(c.Cluster, updateFailure, "failed to update cluster status for cluster %q: %v", c, err)
			return errors.Wrapf(err, "failed to update cluster status for cluster %q", c)
		}
		record.Eventf(c.Cluster, updateSuccess, "updated cluster status for cluster %q", c)
	}

	return nil
}
