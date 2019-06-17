/*
Copyright 2018 The Kubernetes Authors.

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

package constants

import (
	"time"
)

const (
	DefaultBindPort                  = 6443
	ApiServerPort                    = 443
	VmIpAnnotationKey                = "vm-ip-address"
	ControlPlaneVersionAnnotationKey = "control-plane-version"
	KubeletVersionAnnotationKey      = "kubelet-version"
	CreateEventAction                = "Create"
	DeleteEventAction                = "Delete"
	DefaultAPITimeout                = 5 * time.Minute
	VirtualMachineTaskRef            = "current-task-ref"
	KubeadmToken                     = "k8s-token"
	KubeadmTokenExpiryTime           = "k8s-token-expiry-time"
	KubeadmTokenTtl                  = 10 * time.Minute
	KubeadmTokenLeftTime             = 5 * time.Minute
	RequeueAfterSeconds              = 20 * time.Second
	KubeConfigSecretName             = "%s-kubeconfig"
	KubeConfigSecretData             = "admin-kubeconfig"
	VsphereUserKey                   = "username"
	VspherePasswordKey               = "password"
	ClusterIsNullErr                 = "cluster is nil, make sure machines have `clusters.k8s.io/cluster-name` label set and the name references a valid cluster name in the same namespace"
)
