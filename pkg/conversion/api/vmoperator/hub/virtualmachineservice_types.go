/*
Copyright 2026 The Kubernetes Authors.

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

package hub

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conversionmeta "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta"
)

// VirtualMachineServiceType string describes ingress methods for a service.
type VirtualMachineServiceType string

// These types correspond to a subset of the core Service Types.
const (
	// VirtualMachineServiceTypeClusterIP means a service will only be
	// accessible inside the cluster, via the cluster IP.
	VirtualMachineServiceTypeClusterIP VirtualMachineServiceType = "ClusterIP"

	// VirtualMachineServiceTypeLoadBalancer means a service will be exposed via
	// an external load balancer (if the cloud provider supports it), in
	// addition to 'NodePort' type.
	VirtualMachineServiceTypeLoadBalancer VirtualMachineServiceType = "LoadBalancer"

	// VirtualMachineServiceTypeExternalName means a service consists of only a
	// reference to an external name that kubedns or equivalent will return as a
	// CNAME record, with no exposing or proxying of any VirtualMachines
	// involved.
	VirtualMachineServiceTypeExternalName VirtualMachineServiceType = "ExternalName"
)

// VirtualMachineServicePort describes the specification of a service port to
// be exposed by a VirtualMachineService. This VirtualMachineServicePort
// specification includes attributes that define the external and internal
// representation of the service port.
type VirtualMachineServicePort struct {
	// Name describes the name to be used to identify this
	// VirtualMachineServicePort.
	Name string `json:"name"`

	// Protocol describes the Layer 4 transport protocol for this port.
	// Supports "TCP", "UDP", and "SCTP".
	Protocol string `json:"protocol"`

	// Port describes the external port that will be exposed by the service.
	Port int32 `json:"port"`

	// TargetPort describes the internal port open on a VirtualMachine that
	// should be mapped to the external Port.
	TargetPort int32 `json:"targetPort"`
}

// LoadBalancerStatus represents the status of a load balancer.
type LoadBalancerStatus struct {
	// +optional

	// Ingress is a list containing ingress addresses for the load balancer.
	// Traffic intended for the service should be sent to any of these ingress
	// points.
	Ingress []LoadBalancerIngress `json:"ingress,omitempty"`
}

// LoadBalancerIngress represents the status of a load balancer ingress point:
// traffic intended for the service should be sent to an ingress point.
// IP or Hostname may both be set in this structure. It is up to the consumer to
// determine which field should be used when accessing this LoadBalancer.
type LoadBalancerIngress struct {
	// +optional

	// IP is set for load balancer ingress points that are specified by an IP
	// address.
	IP string `json:"ip,omitempty"`

	/*
		// +optional

		// Hostname is set for load balancer ingress points that are specified by a
		// DNS address.
		Hostname string `json:"hostname,omitempty"`
	*/
}

// VirtualMachineServiceSpec defines the desired state of VirtualMachineService.
type VirtualMachineServiceSpec struct {
	// Type specifies a desired VirtualMachineServiceType for this
	// VirtualMachineService. Supported types are ClusterIP, LoadBalancer,
	// ExternalName.
	Type VirtualMachineServiceType `json:"type"`

	// Ports specifies a list of VirtualMachineServicePort to expose with this
	// VirtualMachineService. Each of these ports will be an accessible network
	// entry point to access this service by.
	Ports []VirtualMachineServicePort `json:"ports,omitempty"`

	// +optional

	// Selector specifies a map of key-value pairs, also known as a Label
	// Selector, that is used to match this VirtualMachineService with the set
	// of VirtualMachines that should back this VirtualMachineService.
	Selector map[string]string `json:"selector,omitempty"`
}

// VirtualMachineServiceStatus defines the observed state of
// VirtualMachineService.
type VirtualMachineServiceStatus struct {
	// +optional

	// LoadBalancer contains the current status of the load balancer,
	// if one is present.
	LoadBalancer LoadBalancerStatus `json:"loadBalancer,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=vmservice
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualMachineService is the Schema for the virtualmachineservices API.
type VirtualMachineService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineServiceSpec   `json:"spec,omitempty"`
	Status VirtualMachineServiceStatus `json:"status,omitempty"`

	Source conversionmeta.SourceTypeMeta `json:"source,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// VirtualMachineServiceList contains a list of VirtualMachineService.
type VirtualMachineServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineService `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &VirtualMachineService{}, &VirtualMachineServiceList{})
}
