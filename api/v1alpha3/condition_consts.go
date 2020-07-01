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
)

// Conditions and condition Reasons for the VSphereVM object

const (

	// VMProvisionedCondition documents the status of the provisioning of a VSphereVM.
	VMProvisionedCondition clusterv1.ConditionType = "VMProvisioned"

	// CloningReason documents (Severity=Info) a VSphereVM currently executing the clone operation.
	CloningReason = "Cloning"

	// CloningFailedReason (Severity=Warning) documents a VSphereVM controller detecting
	// an error while provisioning; the reconcile loop will automatically retry the operation,
	// but a user intervention might be required to fix the problem.
	CloningFailedReason = "CloningFailed"

	// PoweringOnReason documents (Severity=Info) a VSphereVM currently executing the power on sequence.
	PoweringOnReason = "PoweringOn"

	// PoweringOnFailedReason (Severity=Warning) documents a VSphereVM controller detecting
	// an error while powering on; the reconcile loop will automatically retry the operation,
	// but a user intervention might be required to fix the problem.
	PoweringOnFailedReason = "PoweringOnFailed"

	// TaskFailure (Severity=Warning) documents a VSphere task failure; the reconcile loop will automatically
	// retry the operation, but a user intervention might be required to fix the problem.
	TaskFailure = "TaskFailure"
)
