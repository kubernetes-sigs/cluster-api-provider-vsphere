/*
Copyright 2021 The Kubernetes Authors.

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

// Package vmoperator contains reconcilers and related functions for VM Operator based VSphereMachines.
package vmoperator

const (
	kubeTopologyZoneLabelKey = "topology.kubernetes.io/zone"

	// ZonePlacementOrgLabelKey is the name of a label that may be used with
	// a shared value across a group of Zone Placement Groups (ZPG), also
	// known as a Zone Placement Org (ZPO). This is to ensure all ZPGs within
	// a ZPO are best-effort spread across available zones.
	//
	// When the number of ZPGs in a ZPO exceed the number of available zones,
	// the ZPGs that are placed after the number of zones are exceeded can end
	// up in any zone.
	ZonePlacementOrgLabelKey = "vmoperator.vmware.com/zone-placement-org"

	// ZonePlacementGroupLabelKey is the name of a label that may be used with
	// a shared value across a group of VMs to ensure all the VMs honor the
	// zone placement result of the first VM from the group to receive a
	// placement recommendation.
	//
	// This label should only be used for concepts like a pool of VMs that
	// otherwise use the same VM class, image and storage class, as placement
	// will only consider these requirements for the first VM that requests
	// placement out of the entire group.
	ZonePlacementGroupLabelKey = "vmoperator.vmware.com/zone-placement-group"

	// ControlPlaneVMClusterModuleGroupName is the name used for the control plane Cluster Module.
	ControlPlaneVMClusterModuleGroupName = "control-plane-group"
	// ClusterModuleNameAnnotationKey is key for the Cluster Module annotation.
	ClusterModuleNameAnnotationKey = "vsphere-cluster-module-group"
	// ProviderTagsAnnotationKey is the key used for the provider tags annotation.
	ProviderTagsAnnotationKey = "vsphere-tag"
	// ControlPlaneVMVMAntiAffinityTagValue is the value used for ProviderTagsAnnotationKey when the machine is a control plane machine.
	ControlPlaneVMVMAntiAffinityTagValue = "CtrlVmVmAATag"
	// WorkerVMVMAntiAffinityTagValue is the value used for ProviderTagsAnnotationKey when the machine is a worker machine.
	WorkerVMVMAntiAffinityTagValue = "WorkerVmVmAATag"
)
