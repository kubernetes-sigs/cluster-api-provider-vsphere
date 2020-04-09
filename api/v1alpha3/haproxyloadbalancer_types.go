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
	"sigs.k8s.io/cluster-api-provider-vsphere/contrib/haproxy/openapi"
)

const (
	// HAProxyLoadBalancerFinalizer allows a reconciler to clean up
	// resources associated with an HAProxyLoadBalancer before removing
	// it from the API server.
	HAProxyLoadBalancerFinalizer = "haproxyloadbalancer.infrastructure.cluster.x-k8s.io"
)

// HAProxyLoadBalancerSpec defines the desired state of HAProxyLoadBalancer.
type HAProxyLoadBalancerSpec struct {
	// VirtualMachineConfiguration is information used to deploy a load balancer
	// VM.
	VirtualMachineConfiguration VirtualMachineCloneSpec `json:"virtualMachineConfiguration"`

	// SSHUser specifies the name of a user that is granted remote access to the
	// deployed VM.
	// +optional
	User *SSHUser `json:"user,omitempty"`

	// ExtraServices is additional services that will proxied to.
	// +optional
	ExtraServices []ExtraService `json:"extraServices,omitempty"`
}

// ExtraServices defines additional services that will be proxied by HAProxyLoadBalancer.
type ExtraService struct {
	// The name of this service
	Name string `json:"name"`

	// The list of ports that are exposed by this service.
	Ports []ExtraServicePort `json:"ports"`

	// Label selector for backend servers. Machines matching
	// the labels will be selected as targets for the backend servers
	// If this is empty, then all the machines are selected by default
	// +optional
	MachineSelector map[string]string `json:"machineSelector,omitempty"`

	// HealthCheck options
	// +optional
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
}

// ExtraServicePort defines the frontend and backend ports used by HAProxyLoadBalancer
type ExtraServicePort struct {
	// Port is the haproxy port that will bind to the service
	Port uint32 `json:"port"`

	// TargetPort is the port that the service is listening on. If not specified use Port.
	// +optional
	TargetPort uint32 `json:"targetPort,omitempty"`
}

// HealthCheck defines the health check
type HealthCheck struct {
	// HealthCheckOption used to perform health check
	HealthCheckOption `json:",inline"`
}

// HealthCheckOption defines the different health checking options available
type HealthCheckOption struct {
	// HTTP health check option
	// +optional
	HTTPCheck *HTTPCheck `json:"httpCheck,omitempty"`
}

// HTTPCheck defines HTTP health check
type HTTPCheck struct {
	// HTTP check object from HAProxy
	// +optional
	openapi.Httpchk `json:",inline"`

	// HTTP scheme to use (HTTP or HTTPS)
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty"`

	// Flag to indicate if certificate needs to be verified
	// +optional
	Verify bool `json:"verify,omitempty"`

	// HTTP check expected response
	// +optional
	Response *HTTPCheckResponse `json:"response,omitempty"`
}

// HTTPCheckResponse defines the different HTTP check output responses
type HTTPCheckResponse struct {
	HTTPCheckResponseOption `json:",inline"`
}

// HTTPCheckResponseOption defines the HTTP check output response options
type HTTPCheckResponseOption struct {
	// HTTP status code response expected
	// +optional
	Status string `json:"status,omitempty"`
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
