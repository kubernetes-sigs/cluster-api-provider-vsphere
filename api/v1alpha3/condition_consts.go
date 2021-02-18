/*
Copyright 2020 The Kubernetes Authors.

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

import clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

// Conditions and condition Reasons for the VSphereCluster object.

const (
	// LoadBalancerAvailableCondition documents the status of the VSphereCluster load balancer.
	LoadBalancerAvailableCondition clusterv1.ConditionType = "LoadBalancerAvailable"

	// LoadBalancerProvisioningReason (Severity=Info) documents a VSphereCluster provisioning a load balancer.
	LoadBalancerProvisioningReason = "LoadBalancerProvisioning"

	// LoadBalancerProvisioningReason (Severity=Warning) documents a VSphereCluster controller detecting
	// while provisioning the load balancer; those kind of errors are usually transient and failed provisioning
	// are automatically re-tried by the controller.
	LoadBalancerProvisioningFailedReason = "LoadBalancerProvisioningFailed"

	// CCMAvailableCondition documents the status of the VSphereCluster cloud controller manager addon.
	CCMAvailableCondition clusterv1.ConditionType = "CCMAvailable"

	// CCMProvisioningFailedReason (Severity=Warning) documents a VSphereCluster controller detecting
	// while installing the cloud controller manager addon; those kind of errors are usually transient
	// the operation is automatically re-tried by the controller.
	CCMProvisioningFailedReason = "CCMProvisioningFailed"

	// CSIAvailableCondition documents the status of the VSphereCluster container storage interface addon.
	CSIAvailableCondition clusterv1.ConditionType = "CSIAvailable"

	// CSIProvisioningFailedReason (Severity=Warning) documents a VSphereCluster controller detecting
	// while installing the container storage interface  addon; those kind of errors are usually transient
	// the operation is automatically re-tried by the controller.
	CSIProvisioningFailedReason = "CSIProvisioningFailed"

	// VCenterAvailableCondition documents the connectivity with vcenter
	// for a given VSphereCluster
	VCenterAvailableCondition clusterv1.ConditionType = "VCenterAvailable"

	// VCenterUnreachableReason (Severity=Error) documents a VSphereCluster controller detecting
	// issues with VCenter reachability;
	VCenterUnreachableReason = "VCenterUnreachable"
)

// Conditions and condition Reasons for the VSphereMachine and the VSphereVM object.
//
// NOTE: VSphereMachine wraps a VMSphereVM, some we are using a unique set of conditions and reasons in order
// to ensure a consistent UX; differences between the two objects will be highlighted in the comments.

const (
	// VMProvisionedCondition documents the status of the provisioning of a VSphereMachine and its underlying VSphereVM.
	VMProvisionedCondition clusterv1.ConditionType = "VMProvisioned"

	// WaitingForClusterInfrastructureReason (Severity=Info) documents a VSphereMachine waiting for the cluster
	// infrastructure to be ready before starting the provisioning process.
	//
	// NOTE: This reason does not apply to VSphereVM (this state happens before the VSphereVM is actually created).
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"

	// WaitingForBootstrapDataReason (Severity=Info) documents a VSphereMachine waiting for the bootstrap
	// script to be ready before starting the provisioning process.
	//
	// NOTE: This reason does not apply to VSphereVM (this state happens before the VSphereVM is actually created).
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"

	// WaitingForStaticIPAllocationReason (Severity=Info) documents a VSphereVM waiting for the allocation of
	// a static IP address.
	WaitingForStaticIPAllocationReason = "WaitingForStaticIPAllocation"

	// CloningReason documents (Severity=Info) a VSphereMachine/VSphereVM currently executing the clone operation.
	CloningReason = "Cloning"

	// CloningFailedReason (Severity=Warning) documents a VSphereMachine/VSphereVM controller detecting
	// an error while provisioning; those kind of errors are usually transient and failed provisioning
	// are automatically re-tried by the controller.
	CloningFailedReason = "CloningFailed"

	// PoweringOnReason documents (Severity=Info) a VSphereMachine/VSphereVM currently executing the power on sequence.
	PoweringOnReason = "PoweringOn"

	// PoweringOnFailedReason (Severity=Warning) documents a VSphereMachine/VSphereVM controller detecting
	// an error while powering on; those kind of errors are usually transient and failed provisioning
	// are automatically re-tried by the controller.
	PoweringOnFailedReason = "PoweringOnFailed"

	// TaskFailure (Severity=Warning) documents a VSphereMachine/VSphere task failure; the reconcile look will automatically
	// retry the operation, but a user intervention might be required to fix the problem.
	TaskFailure = "TaskFailure"

	// WaitingForNetworkAddressesReason (Severity=Info) documents a VSphereMachine waiting for the the machine network
	// settings to be reported after machine being powered on.
	//
	// NOTE: This reason does not apply to VSphereVM (this state happens after the VSphereVM is in ready state).
	WaitingForNetworkAddressesReason = "WaitingForNetworkAddresses"
)
