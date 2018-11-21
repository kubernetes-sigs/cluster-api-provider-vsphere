package govmomi

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	apierrors "sigs.k8s.io/cluster-api/pkg/errors"
	"sigs.k8s.io/cluster-api/pkg/kubeadm"
)

const (
	DefaultSSHPublicKeyFile = "/root/.ssh/vsphere_tmp.pub"
)

func (pv *Provisioner) GetKubeadmToken(cluster *clusterv1.Cluster) (string, error) {
	var token string
	if cluster.ObjectMeta.Annotations != nil {
		if token, ok := cluster.ObjectMeta.Annotations[constants.KubeadmToken]; ok {
			tokenexpirytime, err := time.Parse(time.RFC3339, cluster.ObjectMeta.Annotations[constants.KubeadmTokenExpiryTime])
			if err == nil && tokenexpirytime.Sub(time.Now()) > constants.KubeadmTokenLeftTime {
				return token, nil
			}
		}
	}
	// From the cluster locate the master node
	master, err := vsphereutils.GetMasterForCluster(cluster, pv.lister)
	if err != nil {
		return "", err
	}
	if len(master) == 0 {
		return "", errors.New("No master available")
	}
	kubeconfig, err := pv.GetKubeConfig(cluster)
	if err != nil {
		return "", err
	}
	tmpconfig, err := vsphereutils.CreateTempFile(kubeconfig)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpconfig)
	tokenParams := kubeadm.TokenCreateParams{
		KubeConfig: tmpconfig,
		Ttl:        constants.KubeadmTokenTtl,
	}
	output, err := kubeadm.New().TokenCreate(tokenParams)
	if err != nil {
		return "", fmt.Errorf("unable to create kubeadm token: output - [%s] err - %v", output, err)
	}
	token = strings.TrimSpace(output)
	ncluster := cluster.DeepCopy()
	if ncluster.ObjectMeta.Annotations == nil {
		ncluster.ObjectMeta.Annotations = make(map[string]string)
	}
	ncluster.ObjectMeta.Annotations[constants.KubeadmToken] = token
	// Even though this time might be off by few sec compared to the actual expiry on the token it should not have any impact
	ncluster.ObjectMeta.Annotations[constants.KubeadmTokenExpiryTime] = time.Now().Add(constants.KubeadmTokenTtl).Format(time.RFC3339)
	_, err = pv.clusterV1alpha1.Clusters(cluster.Namespace).Update(ncluster)
	if err != nil {
		glog.Infof("Could not cache the kubeadm token on cluster object: %s", err)
	}
	return token, err
}

// If the Provisioner has a client for updating Machine objects, this will set
// the appropriate reason/message on the Machine.Status. If not, such as during
// cluster installation, it will operate as a no-op. It also returns the
// original error for convenience, so callers can do "return handleMachineError(...)".
func (pv *Provisioner) HandleMachineError(machine *clusterv1.Machine, err *apierrors.MachineError, eventAction string) error {
	if pv.clusterV1alpha1 != nil {
		nmachine := machine.DeepCopy()
		reason := err.Reason
		message := err.Message
		nmachine.Status.ErrorReason = &reason
		nmachine.Status.ErrorMessage = &message
		pv.clusterV1alpha1.Machines(nmachine.Namespace).UpdateStatus(nmachine)
	}
	if eventAction != "" {
		pv.eventRecorder.Eventf(machine, corev1.EventTypeWarning, "Failed"+eventAction, "%v", err.Reason)
	}

	glog.Errorf("Machine error: %v", err.Message)
	return err
}

// If the Provisioner has a client for updating Cluster objects, this will set
// the appropriate reason/message on the Cluster.Status. If not, such as during
// cluster installation, it will operate as a no-op. It also returns the
// original error for convenience, so callers can do "return handleClusterError(...)".
func (pv *Provisioner) HandleClusterError(cluster *clusterv1.Cluster, err *apierrors.ClusterError, eventAction string) error {
	if pv.clusterV1alpha1 != nil {
		ncluster := cluster.DeepCopy()
		reason := err.Reason
		message := err.Message
		ncluster.Status.ErrorReason = reason
		ncluster.Status.ErrorMessage = message
		pv.clusterV1alpha1.Clusters(ncluster.Namespace).UpdateStatus(ncluster)
	}
	if eventAction != "" {
		pv.eventRecorder.Eventf(cluster, corev1.EventTypeWarning, "Failed"+eventAction, "%v", err.Reason)
	}

	glog.Errorf("Cluster error: %v", err.Message)
	return err
}

func (pv *Provisioner) GetSSHPublicKey(cluster *clusterv1.Cluster) (string, error) {
	// First try to read the public key file from the mounted secrets volume
	key, err := ioutil.ReadFile(DefaultSSHPublicKeyFile)
	if err == nil {
		return string(key), nil
	}

	// If the mounted secrets volume not found, try to request it from the API server.
	// TODO(sflxn): We're trying to pull secrets from the default namespace and with name 'sshkeys'.  With
	// the CRD changes, this is no longer the case.  These two values are generated from kustomize.  We
	// need a different way to pass knowledge of the namespace and sshkeys into this container.
	secret, err := pv.k8sClient.Core().Secrets(cluster.Namespace).Get("sshkeys", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data["vsphere_tmp.pub"]), nil
}

func (pv *Provisioner) GetKubeConfig(cluster *clusterv1.Cluster) (string, error) {
	secret, err := pv.k8sClient.Core().Secrets(cluster.Namespace).Get(fmt.Sprintf(constants.KubeConfigSecretName, cluster.UID), metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data[constants.KubeConfigSecretData]), nil
}
