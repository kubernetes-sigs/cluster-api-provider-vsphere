/*
Copyright 2022 The Kubernetes Authors.

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

package clustermodule

import (
	"sort"

	"github.com/blang/semver"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/govmomi/v1beta2"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// Compare returns whether both the cluster module slices are the same.
//
// Compare does not modify its inputs. It is called from a controller watch
// predicate with the ClusterModules of objects coming from the shared informer
// cache, so sorting the input slices in place would mutate cached objects that
// are read concurrently.
func Compare(oldMods, newMods []infrav1.ClusterModule) bool {
	if len(oldMods) != len(newMods) {
		return false
	}

	// Sort copies of the slices to avoid reordering the caller's (potentially
	// cache-backed) slices.
	oldModsSorted := append([]infrav1.ClusterModule(nil), oldMods...)
	newModsSorted := append([]infrav1.ClusterModule(nil), newMods...)

	sort.SliceStable(oldModsSorted, func(i, j int) bool {
		return oldModsSorted[i].TargetObjectName < oldModsSorted[j].TargetObjectName
	})
	sort.SliceStable(newModsSorted, func(i, j int) bool {
		return newModsSorted[i].TargetObjectName < newModsSorted[j].TargetObjectName
	})

	for i := range oldModsSorted {
		if ptr.Deref(oldModsSorted[i].ControlPlane, false) == ptr.Deref(newModsSorted[i].ControlPlane, false) &&
			oldModsSorted[i].TargetObjectName == newModsSorted[i].TargetObjectName &&
			oldModsSorted[i].ModuleUUID == newModsSorted[i].ModuleUUID {
			continue
		}
		return false
	}
	return true
}

// IsClusterCompatible checks if the VCenterVersion is compatibly with CAPV. Only version 7 and over are supported.
func IsClusterCompatible(clusterCtx *capvcontext.ClusterContext) bool {
	version := clusterCtx.VSphereCluster.Status.VCenterVersion
	if version == "" {
		return false
	}
	apiVersion, err := semver.New(string(version))
	if err != nil {
		return false
	}

	return apiVersion.Major >= 7
}
