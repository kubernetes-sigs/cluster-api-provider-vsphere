package govmomi

import (
	"fmt"
	"io/ioutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
)

var (
	DefaultSSHPublicKeyFile = "/root/.ssh/vsphere_tmp.pub"
)

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

func (pv *Provisioner) GetVsphereCredentials(cluster *clusterv1.Cluster) (string, string, error) {
	vsphereConfig, err := v1alpha1.ClusterConfigFromCluster(cluster)
	if err != nil {
		return "", "", err
	}
	// If the vsphereCredentialSecret is specified then read that secret to get the credentials
	if vsphereConfig.VsphereCredentialSecret != "" {
		klog.V(4).Infof("Fetching vsphere credentials from secret %s", vsphereConfig.VsphereCredentialSecret)
		secret, err := pv.k8sClient.Core().Secrets(cluster.Namespace).Get(vsphereConfig.VsphereCredentialSecret, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Error reading secret %s", vsphereConfig.VsphereCredentialSecret)
			return "", "", err
		}
		if username, ok := secret.Data[constants.VsphereUserKey]; ok {
			if password, ok := secret.Data[constants.VspherePasswordKey]; ok {
				return string(username), string(password), nil
			}
		}
		return "", "", fmt.Errorf("Improper secret: Secret %s should have the keys `%s` and `%s` defined in it", vsphereConfig.VsphereCredentialSecret, constants.VsphereUserKey, constants.VspherePasswordKey)
	}
	return vsphereConfig.VsphereUser, vsphereConfig.VspherePassword, nil

}
