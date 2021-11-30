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

package vmoperator

const (
	kubeTopologyZoneLabelKey = "topology.kubernetes.io/zone"

	metadataFormat = `
instance-id: "{{ .Hostname }}"
local-hostname: "{{ .Hostname }}"
{{ if .ControlPlaneEndpoint }}
controlPlaneEndpoint: "{{ .ControlPlaneEndpoint }}"
{{ end }}
`

	ControlPlaneVMClusterModuleGroupName = "control-plane-group"
	ClusterModuleNameAnnotationKey       = "vsphere-cluster-module-group"
	ProviderTagsAnnotationKey            = "vsphere-tag"
	ControlPlaneVMVMAntiAffinityTagValue = "CtrlVmVmAATag"
	WorkerVMVMAntiAffinityTagValue       = "WorkerVmVmAATag"
)
