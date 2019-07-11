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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereClusterProviderSpec is the schema for the vsphereclusterproviderspec API
// +k8s:openapi-gen=true
type VsphereClusterProviderSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Server is the address of the vSphere endpoint.
	Server string `json:"server,omitempty"`

	// Username is the name used to log into the vSphere server.
	//
	// This value is optional unless using clusterctl to bootstrap the initial
	// management cluster.
	// +optional
	Username string `json:"username,omitempty"`

	// Password is the password used to log into the vSphere server.
	//
	// This value is optional unless using clusterctl to bootstrap the initial
	// management cluster.
	// +optional
	Password string `json:"password,omitempty"`

	// Insecure is a flag that controls whether or not to validate the
	// vSphere server's certificate.
	// +optional
	Insecure *bool `json:"insecure,omitempty"`

	// SSHAuthorizedKeys is a list of SSH public keys authorized to access
	// deployed machines.
	//
	// These keys are added to the default user as determined by cloud-init
	// in the images from which the machines are deployed.
	//
	// The default user for CentOS is "centos".
	// The default user for Ubuntu is "ubuntu".
	// +optional
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty"`

	// SSHKeyPair is the SSH key pair for accessing provisioned machines.
	// This field is set at runtime and should not be assigned by an operator.
	// Providing access to existing SSH keys is handled by SSHAuthorizedKeys.
	// Please see SSHAuthorizedKeys for information on accessing the machines.
	// +optional
	SSHKeyPair KeyPair `json:"sshKeyPair,omitempty"`

	// CAKeyPair is the key pair for ca certs.
	// +optional
	CAKeyPair KeyPair `json:"caKeyPair,omitempty"`

	//EtcdCAKeyPair is the key pair for etcd.
	// +optional
	EtcdCAKeyPair KeyPair `json:"etcdCAKeyPair,omitempty"`

	// FrontProxyCAKeyPair is the key pair for FrontProxyKeyPair.
	// +optional
	FrontProxyCAKeyPair KeyPair `json:"frontProxyCAKeyPair,omitempty"`

	// SAKeyPair is the service account key pair.
	// +optional
	SAKeyPair KeyPair `json:"saKeyPair,omitempty"`

	// ClusterConfiguration holds the cluster-wide information used during a
	// kubeadm init call.
	ClusterConfiguration kubeadmv1beta1.ClusterConfiguration `json:"clusterConfiguration,omitempty"`
}

// GetSSHAuthorizedKeys returns a list of all of the public keys authorized to
// access machines via SSH.
// Please note the keys are returned as plain-text.
func (c VsphereClusterProviderSpec) GetSSHAuthorizedKeys() []string {
	keys := []string{}
	for _, k := range c.SSHAuthorizedKeys {
		if len(k) > 0 {
			keys = append(keys, k)
		}
	}
	if len(c.SSHKeyPair.Cert) > 0 {
		keys = append(keys, string(c.SSHKeyPair.Cert))
	}
	return keys
}

// KeyPair is how operators can supply custom keypairs for kubeadm to use.
type KeyPair struct {
	// Cert is the base64-encoded, public side of the key pair.
	// +optional
	Cert []byte `json:"cert,omitempty"`

	// Key is the base64-encoded, private side of the key pair.
	// +optional
	Key []byte `json:"key,omitempty"`
}

// HasCertAndKey returns whether a keypair contains cert and key of non-zero length.
func (kp KeyPair) HasCertAndKey() bool {
	return len(kp.Cert) > 0 && len(kp.Key) > 0
}
func init() {
	SchemeBuilder.Register(&VsphereClusterProviderSpec{})
}
