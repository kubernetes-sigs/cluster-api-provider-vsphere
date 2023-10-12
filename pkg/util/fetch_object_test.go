/*
Copyright 2023 The Kubernetes Authors.

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

package util

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
)

func Test_FetchControlPlaneOwnerObject(t *testing.T) {
	ctx := context.Background()
	kcpName, kcpNs := "test-control-plane", "testing"
	kcp := func(version string) *controlplanev1.KubeadmControlPlane {
		return &controlplanev1.KubeadmControlPlane{
			TypeMeta: metav1.TypeMeta{
				Kind:       "KubeadmControlPlane",
				APIVersion: version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      kcpName,
				Namespace: kcpNs,
			},
		}
	}

	machine := func(ownerRefVersion string) *clusterv1.Machine {
		return &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-machine",
				Namespace: kcpNs,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: controlplanev1.GroupVersion.Group + "/" + ownerRefVersion,
					Kind:       "KubeadmControlPlane",
					Name:       kcpName,
				}},
			},
		}
	}

	tests := []struct {
		name                  string
		kcpOwnerRefAPIVersion string
		storageAPIVersion     string
		noObjects             bool
		hasError              bool
	}{
		{
			name:      "when object is not present",
			noObjects: true,
			hasError:  true,
		},
		{
			name:                  "when object is present with same Group and version",
			kcpOwnerRefAPIVersion: "v1beta1",
			storageAPIVersion:     "v1beta1",
		},
		{
			name:                  "when object is present with same Group but different version",
			kcpOwnerRefAPIVersion: "v1alpha3",
			storageAPIVersion:     "v1beta1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			// The entire manager context is not needed, just the client
			// Instead of rebuilding a scheme and the client reusing the existing helper
			objectsToFetch := []ctrlclient.Object{}
			if !tt.noObjects {
				objectsToFetch = append(objectsToFetch, kcp(tt.storageAPIVersion))
			}
			client := fake.NewControllerManagerContext(objectsToFetch...).Client

			obj, err := FetchControlPlaneOwnerObject(ctx, FetchObjectInput{
				Client: client,
				Object: machine(tt.kcpOwnerRefAPIVersion),
			})
			if tt.hasError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(obj).ToNot(gomega.BeNil())
				g.Expect(obj.GetName()).To(gomega.Equal(kcpName))
			}
		})
	}
}
