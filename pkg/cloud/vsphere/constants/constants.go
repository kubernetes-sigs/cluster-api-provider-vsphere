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
	// DefaultBindPort is the default API port used to generate the kubeadm
	// configurations.
	DefaultBindPort = 6443

	// DefaultRequeue is the default time for how long to wait when
	// requeueing a CAPI operation.
	DefaultRequeue = 20 * time.Second

	// VsphereUserKey is the key used to store/retrieve the vSphere user
	// name from a Kubernetes secret.
	VsphereUserKey = "username"

	// VspherePasswordKey is the key used to store/retrieve the vSphere
	// password from a Kubernetes secret.
	VspherePasswordKey = "password"

	// ReadyAnnotationLabel is the annotation used to indicate a machine and/or
	// cluster are ready.
	ReadyAnnotationLabel = "capv.sigs.k8s.io/ready"
)
