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

package v1alpha3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// HAProxyLoadBalancerFinalizer allows a reconciler to clean up
	// resources associated with an HAProxyLoadBalancer before removing
	// it from the API server.
	HAProxyLoadBalancerFinalizer = "haproxyloadbalancer.infrastructure.cluster.x-k8s.io"
)

// HAProxyLoadBalancerSpec defines the desired state of HAProxyLoadBalancer.
type HAProxyLoadBalancerSpec struct {
	// APIEndpoint is the address and port with which the load balancer API
	// service may be accessed.
	// If omitted then the VirtualMachineConfiguration field is required in
	// order to deploy a new load balancer.
	// +optional
	APIEndpoint APIEndpoint `json:"apiEndpoint,omitempty"`

	// Ports is a list of one or more pairs of ports on which the load balancer
	// listens for incoming traffic and the ports on the backend to which the
	// traffic is transmitted.
	Ports []LoadBalancerPort `json:"ports"`

	// Selector is used to identify the control-plane Machine resources that
	// will be the backend servers for this load balancer.
	Selector metav1.LabelSelector `json:"selector"`

	// CACertificateRef is a reference to a Secret resource that contains the
	// following keys:
	//   * ca.crt - The PEM-encoded, public key for a CA certificate
	//   * ca.key - The PEM-encoded, private key for a CA certificate
	//
	// If unspecified, the Secret's Namespace defaults to
	// HAProxyLoadBalancer.Namespace.
	//
	// If unspecified, the Secret's Name defaults to
	// HAProxyLoadBalancer.Name+"-ca".
	//
	// When using an existing load balancer only the public key is required,
	// however, if the private key is present as well and the
	// ClientCertificateRef does not exist or contain a valid client
	// certificate, then the public and private key in this Secret will be used
	// to generate a valid, client certifiate for an existing load balancer.
	//
	// When provisioning a new load balancer this Secret must contain both
	// a public *and* private key. If the Secret does not exist then a new
	// Secret will be generated with a new CA key pair. If the Secret exists
	// but does not contain a valid CA key pair then a new key pair will be
	// generated and the Secret will be updated.
	//
	// If an existing load balancer is used then the Secret need only to contain
	// the CA's public key.
	CACertificateRef corev1.SecretReference `json:"caCertificateRef"`

	// ClientCredentialsRef is a reference to a Secret resource that contains
	// the following keys:
	//   * client.crt - A PEM-encoded, public key for a client certificate
	//                  used to access the load balancer's API server
	//   * client.key - A PEM-encoded, private key for a client certificate
	//                  used to access the load balancer's API server
	//   * username   - The username used to access the load balancer's API
	//                  server
	//   * password   - The password used to access the load balancer's API
	//                  server
	//
	// If unspecified, the Secret's Namespace defaults to
	// HAProxyLoadBalancer.Namespace.
	//
	// If unspecified, the Secret's Name defaults to
	// HAProxyLoadBalancer.Name+"-client".
	//
	// This Secret must contain both a public *and* private key. If the Secret
	// does not exist then a new Secret will be generated with a new client
	// certificate key pair using the CA from CACertificateRef.
	//
	// If the Secret exists but does not contain a valid client certificate key
	// pair, then a new client certificate key pair will be generated using the
	// CA from CACertificateRef.
	//
	// When the username or password fields are empty, they both default to
	// "guest". The HAProxy load balancer OVA built from the CAPV repository
	// uses mutual certificate validation (client certificates) to control
	// access to the load balancer's API server. However, a username and
	// password are still required, even though they provide no actual access
	// control.
	ClientCredentialsRef corev1.SecretReference `json:"clientCredentialsRef"`

	// VirtualMachineConfiguration is optional information used to deploy a new
	// load VM.
	// If omitted then the APIEndpoint field is required to point to an existing
	// load balancer.
	// +omitempty
	VirtualMachineConfiguration *VirtualMachineCloneSpec `json:"virtualMachineConfiguration,omitempty"`
}

// HAProxyLoadBalancerStatus defines the observed state of HAProxyLoadBalancer.
type HAProxyLoadBalancerStatus struct {
	// Ready indicates whether or not the load balancer is ready.
	//
	// This field is required as part of the Portable Load Balancer model and is
	// inspected via an unstructured reader by other controllers to determine
	// the status of the load balancer.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Address is the IP address or DNS name of the load balancer.
	//
	// This field is required as part of the Portable Load Balancer model and is
	// inspected via an unstructured reader by other controllers to determine
	// the status of the load balancer.
	//
	// +optional
	Address string `json:"address,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=haproxyloadbalancers,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// HAProxyLoadBalancer is the Schema for the haproxyloadbalancers API
type HAProxyLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HAProxyLoadBalancerSpec   `json:"spec,omitempty"`
	Status HAProxyLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HAProxyLoadBalancerList contains a list of HAProxyLoadBalancer
type HAProxyLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HAProxyLoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HAProxyLoadBalancer{}, &HAProxyLoadBalancerList{})
}
