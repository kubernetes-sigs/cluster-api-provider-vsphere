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

package fake

const (
	// ControllerName is the name of the fake controller.
	ControllerName = "fake-controller"

	// ControllerManagerName is the name of the fake controller manager.
	ControllerManagerName = "fake-controller-manager"

	// ControllerManagerNamespace is the name of the namespace in which the
	// fake controller manager's resources are located.
	ControllerManagerNamespace = "fake-capv-system"

	// LeaderElectionNamespace is the namespace used to control leader election
	// for the fake controller manager.
	LeaderElectionNamespace = ControllerManagerNamespace

	// LeaderElectionID is the name of the ID used to control leader election
	// for the fake controller manager.
	LeaderElectionID = ControllerManagerName + "-runtime"

	// Namespace is the fake namespace.
	Namespace = "default"

	// ClusterUUID is the UID of the fake CAPI cluster.
	ClusterUUID = "00000000-0000-0000-0000-000000000002"

	// Clusterv1a2Name is the name of the fake CAPI v1alpha3 Cluster resource.
	Clusterv1a2Name = "fake-cluster"

	// Clusterv1a2UUID is the UID of the fake CAPI v1alpha3 Cluster resource.
	Clusterv1a2UUID = "00000000-0000-0000-0000-000000000000"

	// Machinev1a2Name is the name of the fake CAPI v1alpha3 Machine resource.
	Machinev1a2Name = "fake-machine"

	// Machinev1a2UUID is the UID of the fake CAPI v1alpha3 Machine resource.
	Machinev1a2UUID = "00000000-0000-0000-0000-000000000001"

	VSphereClusterName = "fake-vsphere-cluster"

	// VSphereClusterUUID is the UID of the fake VSphereCluster resource.
	VSphereClusterUUID = "10000000-0000-0000-0000-000000000000"

	// VSphereMachineUUID is the UID of the fake VSphereMachine resource.
	VSphereMachineUUID = "10000000-0000-0000-0000-000000000001"

	// VSphereVMName is the name of the fake VSphereVM resource.
	VSphereVMName = "fake-vm"

	// VSphereVMUUID is the UID of the fake VSphereVMUUID resource.
	VSphereVMUUID = "20000000-0000-0000-0000-000000000002"

	// PodCIDR is the CIDR for the pod network.
	PodCIDR = "1.0.0.0/16"

	// ServiceCIDR is the CIDR for the service network.
	ServiceCIDR = "2.0.0.0/16"
)

var boolTrue = true
