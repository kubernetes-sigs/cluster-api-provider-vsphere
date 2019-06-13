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

// VsphereClusterProviderConfig is the Schema for the vsphereclusterproviderconfigs API
// +k8s:openapi-gen=true
type VsphereClusterProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	VsphereUser             string `json:"vsphereUser,omitempty"`
	VspherePassword         string `json:"vspherePassword,omitempty"`
	VsphereServer           string `json:"vsphereServer"`
	VsphereCredentialSecret string `json:"vsphereCredentialSecret,omitempty"`

	// SSHAuthorizedKeys is a list of SSH public keys authorized to access
	// deployed machines.
	//
	// These keys are added to the default user as determined by cloud-init
	// in the images from which the machines are deployed.
	//
	// The default user for CentOS is "centos".
	// The default user for Ubuntu is "ubuntu".
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty"`

	// CAKeyPair is the key pair for ca certs.
	CAKeyPair KeyPair `json:"caKeyPair,omitempty"`

	//EtcdCAKeyPair is the key pair for etcd.
	EtcdCAKeyPair KeyPair `json:"etcdCAKeyPair,omitempty"`

	// FrontProxyCAKeyPair is the key pair for FrontProxyKeyPair.
	FrontProxyCAKeyPair KeyPair `json:"frontProxyCAKeyPair,omitempty"`

	// SAKeyPair is the service account key pair.
	SAKeyPair KeyPair `json:"saKeyPair,omitempty"`

	// ClusterConfiguration holds the cluster-wide information used during a
	// kubeadm init call.
	ClusterConfiguration kubeadmv1beta1.ClusterConfiguration `json:"clusterConfiguration,omitempty"`
}

// KeyPair is how operators can supply custom keypairs for kubeadm to use.
type KeyPair struct {
	// base64 encoded cert and key
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

// HasCertAndKey returns whether a keypair contains cert and key of non-zero length.
func (kp KeyPair) HasCertAndKey() bool {
	return len(kp.Cert) > 0 && len(kp.Key) > 0
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VsphereClusterProviderConfigList contains a list of VsphereClusterProviderConfig
type VsphereClusterProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VsphereClusterProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VsphereClusterProviderConfig{}, &VsphereClusterProviderConfigList{})
}
