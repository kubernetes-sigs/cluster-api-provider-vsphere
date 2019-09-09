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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=loadbalancers,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// LoadBalancer is the schema for the Load balancer API
type LoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerSpec   `json:"spec,omitempty"`
	Status LoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadBalancerList contains a list of Loadbalancer
type LoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadBalancer `json:"items"`
}

// LoadBalancerSpec defines the desired state of the load balancer
type LoadBalancerSpec struct {

	// Ports ports that should be used in this load balancer
	Ports []LoadBalancerPort `json:"ports"`

	// Selector selects the machines to add to the load balancing pool
	Selector map[string]string `json:"selector"`

	// ConfigRef holds a reference to a provider-specific configuration
	ConfigRef corev1.ObjectReference `json:"configRef"`
}

// LoadBalancerPort describes the desired state of load balancer ports
type LoadBalancerPort struct {

	// Name is the name of this port. This is used by the provider to guarantee uniquness
	// of the <Protocol, Port, TargetPort> association
	Name string `json:"name"`

	// The IP protocol for this port. Supports "TCP" for now.
	// Default is TCP.
	// +optional
	Protocol *Protocol `json:"protocol,omitempty"`

	// The port that will be exposed by this load balancer.
	Port int32 `json:"port"`

	// TargetPort is the port of the machines the load balancer should send traffic to
	// +optional
	TargetPort *int32 `json:"targetPort,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=awsloadbalancerconfigs,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// AWSLoadBalancerConfig is the schema for the aws configuration API
type AWSLoadBalancerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AWSLoadBalancerConfigSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// AWSLoadBalancerConfigList contains a list of AWSLoadBalancerConfig
type AWSLoadBalancerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSLoadBalancerConfig `json:"items"`
}

// AWSLoadBalancerConfigSpec defines the desired state of an AWS load balancer
type AWSLoadBalancerConfigSpec struct {
	// VpcID is the id of the VPC used to create loadBalancers
	VpcID string `json:"vpcId"`

	// SubnetIDs is the list of subnets within the VPC used
	SubnetIDs []string `json:"subnetIds"`

	// Region is the region used for the loadbalancers
	Region string `json:"region"`
}

// Protocol defines listener protocols for a classic load balancer.
type Protocol string

var (
	// ProtocolTCP defines the API string representing the TCP protocol
	ProtocolTCP = Protocol("TCP")
)

// LoadBalancerStatus defines the observed state of the AWS load balancer
type LoadBalancerStatus struct {

	// APIEndpoint represents the endpoint to communicate with the load
	// balancer.
	// +optional
	APIEndpoint APIEndpoint `json:"apiEndpoint,omitempty"`
}

func init() {
	SchemeBuilder.Register(&LoadBalancer{}, &LoadBalancerList{}, &AWSLoadBalancerConfig{}, &AWSLoadBalancerConfigList{})
}
