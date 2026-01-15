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

package client

import (
	"testing"

	. "github.com/onsi/gomega"
	vmoprv1alpha2 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
)

func Test_MergeFrom(t *testing.T) {
	g := NewWithT(t)
	converter.SetTargetVersion(vmoprv1alpha5.GroupVersion.Version)

	cc, err := NewWithConverter(fake.NewClientBuilder().WithScheme(scheme).Build(), converter)
	g.Expect(err).NotTo(HaveOccurred())

	obj := &vmoprvhub.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vm",
			Namespace: "test-ns",
		},
	}

	tests := []struct {
		name      string
		c         client.Client
		obj       client.Object
		wantPatch *conversionMergePatch
		wantErr   bool
	}{
		{
			name: "Creates a patch",
			c:    cc,
			obj:  obj,
			wantPatch: &conversionMergePatch{
				from:   obj,
				client: cc.(*conversionClient),
			},
		},
		{
			name:    "Fails for non conversion client",
			c:       fake.NewClientBuilder().WithScheme(scheme).Build(),
			wantErr: true,
		},
		{
			name:    "Fails for non convertible objects",
			c:       cc,
			obj:     &corev1.Node{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			gotPatch, err := MergeFrom(tt.c, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotPatch).To(Equal(tt.wantPatch))
			g.Expect(gotPatch.Type()).To(Equal(types.MergePatchType))
		})
	}
}

func Test_conversionMergePatch_Data(t *testing.T) {
	g := NewWithT(t)
	cc, err := NewWithConverter(fake.NewClientBuilder().WithScheme(scheme).Build(), converter)
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name          string
		targetVersion string
		patch         *conversionMergePatch
		obj           client.Object
		wantData      []byte
		wantErr       bool
	}{
		{
			name:          "Generates patch data when obj needs conversion to v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			patch: &conversionMergePatch{
				from: &vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-ns",
					},
				},
				client: cc.(*conversionClient),
			},
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantData: []byte(`{"metadata":{"labels":{"foo":"bar"}}}`),
		},
		{
			name:          "Generates patch data when obj is already converted to v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			patch: &conversionMergePatch{
				from: &vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-ns",
					},
				},
				client: cc.(*conversionClient),
			},
			obj: &vmoprv1alpha5.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantData: []byte(`{"metadata":{"labels":{"foo":"bar"}}}`),
		},
		{
			name:          "Generates patch data when obj needs conversion to v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			patch: &conversionMergePatch{
				from: &vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-ns",
					},
				},
				client: cc.(*conversionClient),
			},
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantData: []byte(`{"metadata":{"labels":{"foo":"bar"}}}`),
		},
		{
			name:          "Generates patch data when obj is already converted to v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			patch: &conversionMergePatch{
				from: &vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-ns",
					},
				},
				client: cc.(*conversionClient),
			},
			obj: &vmoprv1alpha2.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantData: []byte(`{"metadata":{"labels":{"foo":"bar"}}}`),
		},
		{
			name:          "Fails when obj is already converted but to a wrong version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			patch: &conversionMergePatch{
				from: &vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-ns",
					},
				},
				client: cc.(*conversionClient),
			},
			obj: &vmoprv1alpha2.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			converter.SetTargetVersion(tt.targetVersion)

			gotData, err := tt.patch.Data(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotData).To(Equal(tt.wantData))
		})
	}
}
