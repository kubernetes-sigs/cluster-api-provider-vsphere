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
	// Ports is a list of one or more pairs of ports on which the load balancer
	// listens for incoming traffic and the ports on the backend to which the
	// traffic is transmitted.
	// +optional
	Ports []LoadBalancerPort `json:"ports,omitempty"`

	// VirtualMachineConfiguration is optional information used to deploy a new
	// load balancer VM.
	// If omitted then the HAProxy API configuration must point to an existing
	// load balancer.
	//
	// +optional
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
