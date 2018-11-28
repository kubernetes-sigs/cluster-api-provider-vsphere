package utils

import (
	"fmt"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"os"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
)

const (
	DefaultKubeConfigEnv = "TEST_KUBECONFIG"
)

type ClusterTools struct {
	kubeconfig string
	Client     *clientset.Clientset
	k8sClient  kubernetes.Interface
}

func NewClusterToolsFromConfig(kubeConfigPath string) (*ClusterTools, error) {
	if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file at %s does not exist", kubeConfigPath)
	}

	ct := &ClusterTools{}

	// Load kubeconfig
	b, err := ioutil.ReadFile(kubeConfigPath)
	if err != nil {
		return nil, err
	}

	// Create a rest config
	rc, err := clientcmd.RESTConfigFromKubeConfig(b)
	if err != nil {
		return nil, err
	}

	c, err := clientset.NewForConfig(rc)
	if err != nil {
		glog.Fatalf("Invalid API configuration for kubeconfig: %v", err)
	}

	ct.Client = c

	k, err := kubernetes.NewForConfig(rest.AddUserAgent(rc, "clustertools"))
	if err != nil {
		glog.Fatalf("Invalid API configuration for kubeconfig: %v", err)
	}

	ct.k8sClient = k

	return ct, nil
}

func NewClusterToolsFromEnv() (*ClusterTools, error) {
	path := os.Getenv(DefaultKubeConfigEnv)
	if path == "" {
		return nil, fmt.Errorf("Could not locate kubeconfig from env var %s", DefaultKubeConfigEnv)
	}

	return NewClusterToolsFromConfig(path)
}

func (c *ClusterTools) PodExist(name, namespace string) bool {
	pod, err := c.k8sClient.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil || pod == nil {
		glog.Errorf("Pod '%s' in namespace '%s' does not exist", name, namespace)
		return false
	}
	return true
}

func (c *ClusterTools) PodRunning(name, namespace string) bool {
	pod, err := c.k8sClient.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil || pod == nil {
		glog.Errorf("Pod '%s' in namespace '%s' does not exist", name, namespace)
		return false
	}
	if pod.Status.Phase == v1.PodRunning {
		return true
	}
	return false
}

func (c *ClusterTools) ClusterExist(name, namespace string) bool {
	cluster, err := c.Client.ClusterV1alpha1().Clusters(namespace).Get(name, metav1.GetOptions{})
	if err != nil || cluster == nil {
		glog.Errorf("Cluster '%s' in namespace '%s' does not exist", name, namespace)
		return false
	}
	return true
}

func (c *ClusterTools) MachineExist(name, namespace string) bool {
	machine, err := c.Client.ClusterV1alpha1().Machines(namespace).Get(name, metav1.GetOptions{})
	if err != nil || machine == nil {
		glog.Errorf("Machine '%s' in namespace '%s' does not exist", name, namespace)
		return false
	}
	return true
}

func (c *ClusterTools) MachineSetExist(name, namespace string) bool {
	machineSet, err := c.Client.ClusterV1alpha1().MachineSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil || machineSet == nil {
		glog.Errorf("MachineSet '%s' in namespace '%s' does not exist", name, namespace)
		return false
	}
	return true
}

func (c *ClusterTools) MachineDeploymentExist(name, namespace string) bool {
	machineDeployment, err := c.Client.ClusterV1alpha1().MachineDeployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil || machineDeployment == nil {
		glog.Errorf("MachineDeployment '%s' in namespace '%s' does not exist", name, namespace)
		return false
	}
	return true
}

func (c *ClusterTools) ClusterCount(namespace string) int {
	list, err := c.Client.ClusterV1alpha1().Clusters(namespace).List(metav1.ListOptions{})
	if err != err || list == nil {
		return 0
	}

	return len(list.Items)
}

func (c *ClusterTools) MachineCount(namespace string) int {
	list, err := c.Client.ClusterV1alpha1().Machines(namespace).List(metav1.ListOptions{})
	if err != err || list == nil {
		return 0
	}

	return len(list.Items)
}

func (c *ClusterTools) MachineSetsCount(namespace string) int {
	list, err := c.Client.ClusterV1alpha1().MachineSets(namespace).List(metav1.ListOptions{})
	if err != err || list == nil {
		return 0
	}

	return len(list.Items)
}

func (c *ClusterTools) MachineDeploymentsCount(namespace string) int {
	list, err := c.Client.ClusterV1alpha1().MachineDeployments(namespace).List(metav1.ListOptions{})
	if err != err || list == nil {
		return 0
	}

	return len(list.Items)
}

func (c *ClusterTools) MachineHasIp(name, namespace string) (string, bool) {
	machine, err := c.Client.ClusterV1alpha1().Machines(namespace).Get(name, metav1.GetOptions{})
	if err != nil || machine == nil {
		glog.Errorf("Machine '%s' in namespace '%s' does not exist", name, namespace)
		return "", false
	}

	if ip, ok := machine.Annotations["vm-ip-address"]; ok {
		return ip, true
	}

	return "", false
}

func (c *ClusterTools) AllMachinesHaveIp(namespace string) bool {
	list, err := c.Client.ClusterV1alpha1().Machines(namespace).List(metav1.ListOptions{})
	if err != err || list == nil {
		return false
	}

	haveIPs := true

	for _, m := range list.Items {
		if _, ok := m.Annotations["vm-ip-address"]; !ok {
			haveIPs = false
			break
		}
	}

	return haveIPs
}