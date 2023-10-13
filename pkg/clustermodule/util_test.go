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
	"testing"

	"github.com/google/uuid"
	"github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

func Test_IsCompatible(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		isCompatible bool
	}{
		{
			name:    "incorrect version",
			version: "foo",
		},
		{
			name:    "empty version",
			version: "",
		},
		{
			name:    "incompatible version",
			version: "6.7.0",
		},
		{
			name:         "compatible version",
			version:      "7.0.3",
			isCompatible: true,
		},
		{
			name:         "next compatible version",
			version:      "8.0.0",
			isCompatible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &infrav1.VSphereCluster{
				Status: infrav1.VSphereClusterStatus{
					VCenterVersion: infrav1.NewVCenterVersion(tt.version),
				},
			}

			g := gomega.NewWithT(t)
			isCompatible := IsClusterCompatible(&capvcontext.ClusterContext{
				VSphereCluster: cluster,
			})
			g.Expect(isCompatible).To(gomega.Equal(tt.isCompatible))
		})
	}
}

func Test_Compare(t *testing.T) {
	clusterMod := func(isControlPlane bool, objName, uuid string) infrav1.ClusterModule {
		return infrav1.ClusterModule{
			ControlPlane:     isControlPlane,
			TargetObjectName: objName,
			ModuleUUID:       uuid,
		}
	}

	uuidOne, uuidTwo := uuid.New().String(), uuid.New().String()

	tests := []struct {
		name     string
		old, new []infrav1.ClusterModule
		isSame   bool
	}{
		{
			name: "different lengths for module slices",
			old:  []infrav1.ClusterModule{clusterMod(true, "foo", uuidOne)},
			new: []infrav1.ClusterModule{
				clusterMod(true, "foo", uuidOne),
				clusterMod(false, "bar", uuidTwo),
			},
		},
		{
			name: "same length but different objects",
			old: []infrav1.ClusterModule{
				clusterMod(true, "foo", uuidOne),
				clusterMod(false, "baz", uuidTwo),
			},
			new: []infrav1.ClusterModule{
				clusterMod(true, "foo", uuidOne),
				clusterMod(false, "bar", uuidTwo),
			},
		},
		{
			name: "same objects with same input order",
			old: []infrav1.ClusterModule{
				clusterMod(true, "foo", uuidOne),
				clusterMod(false, "baz", uuidTwo),
			},
			new: []infrav1.ClusterModule{
				clusterMod(true, "foo", uuidOne),
				clusterMod(false, "baz", uuidTwo),
			},
			isSame: true,
		},
		{
			name: "same objects with different input order",
			old: []infrav1.ClusterModule{
				clusterMod(true, "foo", uuidOne),
				clusterMod(false, "baz", uuidTwo),
			},
			new: []infrav1.ClusterModule{
				clusterMod(false, "baz", uuidTwo),
				clusterMod(true, "foo", uuidOne),
			},
			isSame: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			g.Expect(Compare(tt.old, tt.new)).To(gomega.Equal(tt.isSame))
		})
	}
}
