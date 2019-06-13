package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientv1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	capierr "sigs.k8s.io/cluster-api/pkg/controller/error"
	"sigs.k8s.io/cluster-api/pkg/util"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/certificates"
)

const (
	defaultBindPort = 6443
)

// GetMachineRole returns either "controlplane", "node", or an empty
// string if the role could be ascertained.
func GetMachineRole(machine *clusterv1.Machine) string {
	set := machine.Labels["set"]
	if ok, _ := regexp.MatchString(`(?i)controlplane|master`, set); ok {
		return "controlplane"
	}
	if ok, _ := regexp.MatchString(`(?i)(?:worker-)?node`, set); ok {
		return "node"
	}
	nodeType := machine.Labels["node-type"]
	if ok, _ := regexp.MatchString(`(?i)(?:worker-)?node`, nodeType); ok {
		return "node"
	}
	return ""
}

// byMachineCreatedTimestamp implements sort.Interface for []clusterv1.Machine
// based on the machine's creation timestamp.
type byMachineCreatedTimestamp []*clusterv1.Machine

func (a byMachineCreatedTimestamp) Len() int      { return len(a) }
func (a byMachineCreatedTimestamp) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byMachineCreatedTimestamp) Less(i, j int) bool {
	return a[i].CreationTimestamp.Before(&a[j].CreationTimestamp)
}

// GetControlPlaneEndpoint returns the control plane endpoint for the cluster.
// If a control plane endpoint was specified in the cluster configuration, then
// that value will be returned.
// Otherwise this function will return the endpoint of the first control plane
// node in the cluster that reports an IP address.
// If no control plane nodes have reported an IP address then this function
// returns an error.
func GetControlPlaneEndpoint(
	cluster *clusterv1.Cluster,
	client clientv1.Interface) (string, error) {

	if len(cluster.Status.APIEndpoints) > 0 {
		controlPlaneEndpoint := net.JoinHostPort(
			cluster.Status.APIEndpoints[0].Host,
			strconv.Itoa(cluster.Status.APIEndpoints[0].Port))
		klog.V(2).Infof("got control plane endpoint from cluster APIEndpoints "+
			" %s=%s %s=%s %s=%s",
			"control-plane-endpoint", controlPlaneEndpoint,
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
		return controlPlaneEndpoint, nil
	}

	clusterProviderConfig, err := GetClusterProviderSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return "", errors.Wrapf(
			err,
			"unable to get cluster provider config while searching for "+
				"control plane endpoint %s=%s %s=%s",
			"cluster-name", cluster.Name,
			"cluster-namespace", cluster.Namespace)
	}
	if controlPlaneEndpoint := clusterProviderConfig.ClusterConfiguration.ControlPlaneEndpoint; controlPlaneEndpoint != "" {
		klog.V(2).Infof("got control plane endpoint from cluster config %s=%s %s=%s %s=%s",
			"control-plane-endpoint", controlPlaneEndpoint,
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
		return controlPlaneEndpoint, nil
	}

	if client == nil {
		return "", errors.Errorf(
			"cluster client is nil while searching for "+
				"control plane endpoint %s=%s %s=%s",
			"cluster-name", cluster.Name,
			"cluster-namespace", cluster.Namespace)
	}

	controlPlaneMachines, err := GetControlPlaneMachinesForCluster(cluster, client)
	if err != nil {
		return "", errors.Wrapf(
			err,
			"unable to get control plane machines while searching for "+
				"control plane endpoint %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
	}

	if len(controlPlaneMachines) == 0 {
		return "", errors.Wrapf(
			&capierr.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds},
			"no control plane machines defined while searching for "+
				"control plane endpoint %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
	}

	// Sort the control plane machines so the first one created is always the
	// one used to provide the address for the control plane endpoint.
	sortedControlPlaneMachines := byMachineCreatedTimestamp(controlPlaneMachines)
	sort.Sort(sortedControlPlaneMachines)

	machine := sortedControlPlaneMachines[0]
	machineIPAddr, _ := GetIP(nil, machine)
	if machineIPAddr == "" {
		return "", errors.Wrapf(
			&capierr.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds},
			"first control plane machine has not reported "+
				"network address while searching for control plane endpoint "+
				"%s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	machineProviderConfig, err := GetMachineProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return "", errors.Wrapf(
			err,
			"unable to get machine provider config while searching for "+
				"control plane endpoint %s=%s %s=%s %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			"machine-namespace", machine.Namespace,
			"machine-name", machine.Name)
	}

	// Check both the Init and Join config for the bind port. If the Join config
	// has a bind port that is different then use it.
	bindPort := GetAPIServerBindPort(machineProviderConfig)

	controlPlaneEndpoint := net.JoinHostPort(
		machineIPAddr, strconv.Itoa(int(bindPort)))

	klog.V(2).Infof("got control plane endpoint from machine config "+
		"%s=%s %s=%s %s=%s %s=%s %s=%s",
		"control-plane-endpoint", controlPlaneEndpoint,
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name)

	return controlPlaneEndpoint, nil
}

// GetAPIServerBindPort returns the APIServer bind port for a node
// joining the cluster.
func GetAPIServerBindPort(machineConfig *v1alpha1.VsphereMachineProviderConfig) int32 {
	bindPort := machineConfig.KubeadmConfiguration.Init.LocalAPIEndpoint.BindPort
	if cp := machineConfig.KubeadmConfiguration.Join.ControlPlane; cp != nil {
		if jbp := cp.LocalAPIEndpoint.BindPort; jbp != bindPort {
			bindPort = jbp
		}
	}
	if bindPort == 0 {
		bindPort = defaultBindPort
	}
	return bindPort
}

// GetKubeConfig returns the kubeconfig for the given cluster.
func GetKubeConfig(
	cluster *clusterv1.Cluster,
	client clientv1.Interface) (string, error) {

	// Load provider config.
	config, err := GetClusterProviderSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return "", errors.Wrapf(
			err,
			"unable to get cluster provider spec for cluster "+
				"while gettig kubeconfig %s=%s %s=%s",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name)
	}

	cert, err := certificates.DecodeCertPEM(config.CAKeyPair.Cert)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode CA Cert")
	} else if cert == nil {
		return "", errors.New("certificate not found in config")
	}

	key, err := certificates.DecodePrivateKeyPEM(config.CAKeyPair.Key)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode private key")
	} else if key == nil {
		return "", errors.New("key not found in status")
	}

	controlPlaneEndpoint, err := GetControlPlaneEndpoint(cluster, client)
	if err != nil {
		return "", err
	}

	server := fmt.Sprintf("https://%s", controlPlaneEndpoint)

	cfg, err := certificates.NewKubeconfig(cluster.Name, server, cert, key)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate a kubeconfig")
	}

	yaml, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", errors.Wrap(err, "failed to serialize config to yaml")
	}

	return string(yaml), nil
}

// GetControlPlaneStatus returns a flag indicating whether or not the cluster
// is online.
// If the flag is true then the second return value is the cluster's control
// plane endpoint.
func GetControlPlaneStatus(
	cluster *clusterv1.Cluster,
	client clientv1.Interface) (bool, string, error) {

	if err := getControlPlaneStatus(cluster, client); err != nil {
		return false, "", errors.Wrapf(
			&capierr.RequeueAfterError{RequeueAfter: constants.RequeueAfterSeconds},
			"unable to get control plane status %s=%s %s=%s: %v",
			"cluster-namespace", cluster.Namespace,
			"cluster-name", cluster.Name,
			err)
	}
	controlPlaneEndpoint, _ := GetControlPlaneEndpoint(cluster, client)
	return true, controlPlaneEndpoint, nil
}

func getControlPlaneStatus(
	cluster *clusterv1.Cluster,
	client clientv1.Interface) error {

	clientSet, err := GetKubeClientForCluster(cluster, client)
	if err != nil {
		return err
	}

	if _, err := clientSet.Nodes().List(metav1.ListOptions{}); err != nil {
		return errors.Wrapf(err, "unable to list nodes")
	}

	return nil
}

// GetKubeClientForCluster returns a Kubernetes client for the given cluster.
func GetKubeClientForCluster(
	cluster *clusterv1.Cluster,
	client clientv1.Interface) (corev1.CoreV1Interface, error) {

	kubeconfig, err := GetKubeConfig(cluster, client)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get kubeconfig")
	}

	clientConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to create client config")
	}

	return corev1.NewForConfig(clientConfig)
}

// GetControlPlaneMachinesForCluster returns the control plane nodes for the given cluster
func GetControlPlaneMachinesForCluster(cluster *clusterv1.Cluster, lister clientv1.Interface) ([]*clusterv1.Machine, error) {
	labelSet := labels.Set(map[string]string{
		clusterv1.MachineClusterLabelName: cluster.Name,
	})

	machines, err := lister.Machines().Lister().Machines(cluster.Namespace).List(labelSet.AsSelector())
	if err != nil {
		return nil, err
	}

	controlPlaneMachines := make([]*clusterv1.Machine, 0)
	for _, machine := range machines {
		if !util.IsControlPlaneMachine(machine) {
			continue
		}

		controlPlaneMachines = append(controlPlaneMachines, machine)
	}

	return controlPlaneMachines, nil
}

func GetIP(_ *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	if machine.ObjectMeta.Annotations == nil {
		return "", errors.New("could not get IP")
	}
	if ip, ok := machine.ObjectMeta.Annotations[constants.VmIpAnnotationKey]; ok {
		return ip, nil
	}
	return "", errors.New("could not get IP")
}

func GetMachineProviderStatus(machine *clusterv1.Machine) (*v1alpha1.VsphereMachineProviderStatus, error) {
	if machine.Status.ProviderStatus == nil {
		return nil, nil
	}
	status := &v1alpha1.VsphereMachineProviderStatus{}
	err := json.Unmarshal(machine.Status.ProviderStatus.Raw, status)
	if err != nil {
		klog.V(4).Infof("error unmarshaling machine provider status: %s", err.Error())
		return nil, err
	}
	return status, nil
}

func GetClusterProviderStatus(cluster *clusterv1.Cluster) (*v1alpha1.VsphereClusterProviderStatus, error) {
	if cluster.Status.ProviderStatus == nil {
		return nil, nil
	}
	status := &v1alpha1.VsphereClusterProviderStatus{}
	err := json.Unmarshal(cluster.Status.ProviderStatus.Raw, status)
	if err != nil {
		klog.V(4).Infof("error unmarshaling cluster provider status: %s", err.Error())

		return nil, err
	}
	return status, nil
}

func GetMachineProviderSpec(providerSpec clusterv1.ProviderSpec) (*v1alpha1.VsphereMachineProviderConfig, error) {
	config := &v1alpha1.VsphereMachineProviderConfig{}

	if providerSpec.Value == nil {
		return nil, fmt.Errorf("machine providerconfig is invalid (nil)")
	}

	err := yaml.Unmarshal(providerSpec.Value.Raw, config)
	if err != nil {
		return nil, fmt.Errorf("machine providerconfig unmarshalling failure: %s", err.Error())
	}
	return config, nil
}

func GetClusterProviderSpec(providerSpec clusterv1.ProviderSpec) (*v1alpha1.VsphereClusterProviderConfig, error) {
	config := &v1alpha1.VsphereClusterProviderConfig{}

	if providerSpec.Value == nil {
		return nil, fmt.Errorf("cluster providerconfig is invalid (nil)")
	}

	err := yaml.Unmarshal(providerSpec.Value.Raw, config)
	if err != nil {
		return nil, fmt.Errorf("cluster providerconfig unmarshalling failure: %s", err.Error())
	}
	return config, nil
}

// Just a temporary hack to grab a single range from the config.
func GetSubnet(netRange clusterv1.NetworkRanges) string {
	if len(netRange.CIDRBlocks) == 0 {
		return ""
	}
	return netRange.CIDRBlocks[0]
}

func GetMachineRef(machine *clusterv1.Machine) (string, error) {
	pc, err := GetMachineProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return "", err
	}
	return pc.MachineRef, nil
}

func GetActiveTasks(machine *clusterv1.Machine) string {
	ps, err := GetMachineProviderStatus(machine)
	if err != nil || ps == nil {
		return ""
	}
	return ps.TaskRef
}

func CreateTempFile(contents string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		klog.Warningf("Error creating temporary file")
		return "", err
	}
	// For any error in this method, clean up the temp file
	defer func(pErr *error, path string) {
		if *pErr != nil {
			if err := os.Remove(path); err != nil {
				klog.Warningf("Error removing file '%s': %v", path, err)
			}
		}
	}(&err, tmpFile.Name())

	if _, err = tmpFile.Write([]byte(contents)); err != nil {
		klog.Warningf("Error writing to temporary file '%s'", tmpFile.Name())
		return "", err
	}
	if err = tmpFile.Close(); err != nil {
		return "", err
	}
	if err = os.Chmod(tmpFile.Name(), 0644); err != nil {
		klog.Warningf("Error setting file permission to 0644 for the temporary file '%s'", tmpFile.Name())
		return "", err
	}
	return tmpFile.Name(), nil
}

// ByteToGiB returns n/1024^3. The input must be an integer that can be
// appropriately divisible.
func ByteToGiB(n int64) int64 {
	return n / int64(math.Pow(1024, 3))
}

// GiBToByte returns n*1024^3.
func GiBToByte(n int64) int64 {
	return int64(n * int64(math.Pow(1024, 3)))
}

func IsValidUUID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}

func GetNextBackOff() time.Duration {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = constants.RequeueAfterSeconds
	b.MaxInterval = constants.RequeueAfterSeconds + 10*time.Second
	b.Reset()
	return b.NextBackOff()
}
