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

// Package api defines the hub version of supervisor types and conversion to the corresponding spoke types.
package api //nolint:revive

import (
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	vmoprv1alpha2conversion "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/v1alpha2"
	vmoprv1alpha5conversion "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/v1alpha5"
)

// DefaultConverter is a converter aware of the API types and the conversions defined in sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api.
var DefaultConverter *conversion.Converter

func init() {
	DefaultConverter = conversion.NewConverter()

	utilruntime.Must(vmoprvhub.AddToConverter(DefaultConverter))
	utilruntime.Must(vmoprv1alpha2conversion.AddToConverter(DefaultConverter))
	utilruntime.Must(vmoprv1alpha5conversion.AddToConverter(DefaultConverter))

	// TODO: Add dynamic selection of target version.
	DefaultConverter.SetTargetVersion(vmoprv1alpha5.GroupVersion.Version)
}
