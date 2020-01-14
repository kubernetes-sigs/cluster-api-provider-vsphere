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
	// NSXTLoadBalancerFinalizer allows a reconciler to clean up
	// resources associated with an NSXTLoadBalancer before removing
	// it from the API server.
	NSXTLoadBalancerFinalizer = "nsxtloadbalancer.infrastructure.cluster.x-k8s.io"
)

// NSXTLoadBalancerSpec defines the desired state of NSXTLoadBalancer.
type NSXTLoadBalancerSpec struct {
	// lbServiceID is the ID of the NSX-T LoadBalancer Service where virtual servers
	// for Service Type=LoadBalancer are created
	LoadBalancerServiceID string `json:"loadBalancerServiceID"`

	// vipPoolID is the ID of the IP Pool where VIPs will be allocated
	VirtualIPPoolID string `json:"virtualIPPoolID"`

	// Server is the address of the NSX-T endpoint
	Server string `json:"server"`

	// Insecure is a flag that controls whether or not to validate the
	// NSX-T endpoint's certificate.
	// +optional
	Insecure bool `json:"insecure,omitempty"`

}

// NSXTLoadBalancerStatus defines the observed state of NSXTLoadBalancer.
type NSXTLoadBalancerStatus struct {
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
// +kubebuilder:resource:path=nsxtloadbalancers,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// NSXTLoadBalancer is the Schema for the nsxtLoadBalancerStatus API
type NSXTLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NSXTLoadBalancerSpec   `json:"spec,omitempty"`
	Status NSXTLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NSXTLoadBalancerList contains a list of NSXTLoadBalancer
type NSXTLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NSXTLoadBalancer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NSXTLoadBalancer{}, &NSXTLoadBalancerList{})
}
