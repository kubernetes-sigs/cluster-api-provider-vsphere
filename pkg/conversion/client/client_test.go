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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	vmoprv1alpha2 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1applyconfigurations "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion"
	conversionapi "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api"
	conversionmeta "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
)

var (
	scheme            = runtime.NewScheme()
	v1alpha2Converter *conversion.Converter
	v1alpha5Converter *conversion.Converter
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(vmoprvhub.AddToScheme(scheme))
	utilruntime.Must(vmoprv1alpha2.AddToScheme(scheme))
	utilruntime.Must(vmoprv1alpha5.AddToScheme(scheme))

	v1alpha2Converter = conversionapi.DefaultConverterFor(vmoprv1alpha2.GroupVersion)
	v1alpha5Converter = conversionapi.DefaultConverterFor(vmoprv1alpha5.GroupVersion)
}

func converterForVersion(v string) *conversion.Converter {
	switch v {
	case vmoprv1alpha2.GroupVersion.Version:
		return v1alpha2Converter
	case vmoprv1alpha5.GroupVersion.Version:
		return v1alpha5Converter
	}
	panic("unknown version")
}

func Test_conversionClient_Get(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		wantObj       client.Object
	}{
		{
			name:          "Get VirtualMachine when target version is v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj: &vmoprv1alpha2.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
			},
			wantObj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha2.GroupVersion.String(),
				},
			},
		},
		{
			name:          "Get VirtualMachine when target version is v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprv1alpha5.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
			},
			wantObj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha5.GroupVersion.String(),
				},
			},
		},
		{
			name:          "Get non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-n",
					Namespace: "test-ns",
				},
			},
			wantObj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-n",
					Namespace: "test-ns",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.obj).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			gvk, err := c.internalClient.GroupVersionKindFor(tt.wantObj)
			g.Expect(err).NotTo(HaveOccurred())

			o, err := c.internalClient.Scheme().New(gvk)
			g.Expect(err).NotTo(HaveOccurred())

			gotObj := o.(client.Object)
			err = c.Get(t.Context(), client.ObjectKeyFromObject(tt.obj), gotObj)
			g.Expect(err).NotTo(HaveOccurred())

			tt.wantObj.SetResourceVersion(gotObj.GetResourceVersion())
			g.Expect(gotObj).To(Equal(tt.wantObj))
		})
	}
}

func Test_conversionClient_List(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		objs          []client.Object
		wantObjs      []client.Object
	}{
		{
			name:          "List VirtualMachines when target version is v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			objs: []client.Object{
				&vmoprv1alpha2.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm1",
						Namespace: "test-ns",
					},
				},
				&vmoprv1alpha2.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm2",
						Namespace: "test-ns",
					},
				},
			},
			wantObjs: []client.Object{
				&vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm1",
						Namespace: "test-ns",
					},
					Source: conversionmeta.SourceTypeMeta{
						APIVersion: vmoprv1alpha2.GroupVersion.String(),
					},
				},
				&vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm2",
						Namespace: "test-ns",
					},
					Source: conversionmeta.SourceTypeMeta{
						APIVersion: vmoprv1alpha2.GroupVersion.String(),
					},
				},
			},
		},
		{
			name:          "List VirtualMachines when target version is v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			objs: []client.Object{
				&vmoprv1alpha5.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm1",
						Namespace: "test-ns",
					},
				},
				&vmoprv1alpha5.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm2",
						Namespace: "test-ns",
					},
				},
			},
			wantObjs: []client.Object{
				&vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm1",
						Namespace: "test-ns",
					},
					Source: conversionmeta.SourceTypeMeta{
						APIVersion: vmoprv1alpha5.GroupVersion.String(),
					},
				},
				&vmoprvhub.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm2",
						Namespace: "test-ns",
					},
					Source: conversionmeta.SourceTypeMeta{
						APIVersion: vmoprv1alpha5.GroupVersion.String(),
					},
				},
			},
		},
		{
			name:          "List non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			objs: []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-n1",
						Namespace: "test-ns",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-n2",
						Namespace: "test-ns",
					},
				},
			},
			wantObjs: []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-n1",
						Namespace: "test-ns",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-n2",
						Namespace: "test-ns",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := &conversionClient{
				internalClient: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objs...).Build(),
				converter:      converterForVersion(tt.targetVersion),
			}

			gvk, err := c.internalClient.GroupVersionKindFor(tt.wantObjs[0])
			g.Expect(err).NotTo(HaveOccurred())

			gvk.Kind = fmt.Sprintf("%sList", gvk.Kind)

			o, err := c.internalClient.Scheme().New(gvk)
			g.Expect(err).NotTo(HaveOccurred())

			gotObjList := o.(client.ObjectList)
			err = c.List(t.Context(), gotObjList)
			g.Expect(err).NotTo(HaveOccurred())

			gotItems, err := meta.ExtractList(gotObjList)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(gotItems).To(HaveLen(len(tt.wantObjs)))
			for i, wantHubObj := range tt.wantObjs {
				gotItem := gotItems[i].(client.Object)
				wantHubObj.SetResourceVersion(gotItem.GetResourceVersion())
				g.Expect(gotItem).To(Equal(wantHubObj))
			}
		})
	}
}

func Test_conversionClient_Create(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		wantErr       bool
	}{
		{
			name:          "Create VirtualMachine when target version is v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
			},
		},
		{
			name:          "Create VirtualMachine when target version is v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
			},
		},
		{
			name:          "Create non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-n",
					Namespace: "test-ns",
				},
			},
		},
		{
			name:          "Accepts Source.APIVersion when equal to target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha5.GroupVersion.String(),
				},
			},
		},
		{
			name:          "Fails when Source.APIVersion different from target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha2.GroupVersion.String(),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			c := &conversionClient{
				internalClient: fake.NewClientBuilder().WithScheme(scheme).Build(),
				converter:      converterForVersion(tt.targetVersion),
			}

			objOriginal := tt.obj.DeepCopyObject().(client.Object)

			err := c.Create(ctx, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(tt.obj.GetResourceVersion()).To(BeEmpty())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(tt.obj.GetResourceVersion()).ToNot(BeEmpty())

			gvk, err := c.internalClient.GroupVersionKindFor(tt.obj)
			g.Expect(err).NotTo(HaveOccurred())

			o, err := c.internalClient.Scheme().New(gvk)
			g.Expect(err).NotTo(HaveOccurred())

			gotObj := o.(client.Object)
			err = c.Get(ctx, client.ObjectKeyFromObject(tt.obj), gotObj)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(gotObj.GetResourceVersion()).ToNot(BeEmpty())
			objOriginal.SetResourceVersion(gotObj.GetResourceVersion())
			if convertible, isConvertible := gotObj.(conversionmeta.Convertible); isConvertible {
				g.Expect(convertible.GetSource().APIVersion).To(Equal(schema.GroupVersion{Group: gvk.Group, Version: tt.targetVersion}.String()))
				convertible.SetSource(conversionmeta.SourceTypeMeta{})
			}
			if convertible, isConvertible := objOriginal.(conversionmeta.Convertible); isConvertible {
				convertible.SetSource(conversionmeta.SourceTypeMeta{})
			}
			g.Expect(gotObj).To(Equal(objOriginal))
		})
	}
}

func Test_conversionClient_Delete(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		wantErr       bool
	}{
		{
			name:          "Delete VirtualMachine when target version is v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
			},
		},
		{
			name:          "Delete VirtualMachine when target version is v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
			},
		},
		{
			name:          "Delete non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-n",
					Namespace: "test-ns",
				},
			},
		},
		{
			name:          "Accepts Source.APIVersion when equal to target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha5.GroupVersion.String(),
				},
			},
		},
		{
			name:          "Fails when Source.APIVersion different from target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha2.GroupVersion.String(),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			tmpSourceVersion := ""
			if convertible, isConvertible := tt.obj.(conversionmeta.Convertible); isConvertible {
				tmpSourceVersion = convertible.GetSource().APIVersion
				convertible.SetSource(conversionmeta.SourceTypeMeta{})
			}

			err = c.Create(ctx, tt.obj)
			g.Expect(err).NotTo(HaveOccurred())

			if convertible, isConvertible := tt.obj.(conversionmeta.Convertible); isConvertible {
				convertible.SetSource(conversionmeta.SourceTypeMeta{APIVersion: tmpSourceVersion})
			}

			err = c.Delete(ctx, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			gvk, err := c.internalClient.GroupVersionKindFor(tt.obj)
			g.Expect(err).NotTo(HaveOccurred())

			o, err := c.internalClient.Scheme().New(gvk)
			g.Expect(err).NotTo(HaveOccurred())

			gotObj := o.(client.Object)
			err = c.Get(ctx, client.ObjectKeyFromObject(tt.obj), gotObj)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	}
}

func Test_conversionClient_Update(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		wantErr       bool
	}{
		{
			name:          "Update non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-n",
					Namespace: "test-ns",
				},
			},
		},
		{
			name:          "Return error for hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())

			c := cc.(*conversionClient)

			err = c.Update(ctx, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})
	}
}

func Test_conversionClient_Patch(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		modifyFunc    func(client.Object) client.Object
		wantErr       bool
	}{
		{
			name:          "Patch VirtualMachine when target version is v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Spec.ClassName = "another-class"
				return vm
			},
		},
		{
			name:          "Patch VirtualMachine when target version is v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Spec.ClassName = "another-class"
				return vm
			},
		},
		{
			name:          "Patch non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-n",
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				n := o.(*corev1.Node)
				n.Spec.ProviderID = "foo"
				return n
			},
		},
		{
			name:          "Accepts Source.APIVersion when equal to target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha5.GroupVersion.String(),
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Spec.ClassName = "another-class"
				return vm
			},
		},
		{
			name:          "Fails when Source.APIVersion different from target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha2.GroupVersion.String(),
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Spec.ClassName = "another-class"
				return vm
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			tmpSourceVersion := ""
			if convertible, isConvertible := tt.obj.(conversionmeta.Convertible); isConvertible {
				tmpSourceVersion = convertible.GetSource().APIVersion
				convertible.SetSource(conversionmeta.SourceTypeMeta{})
			}

			err = c.Create(ctx, tt.obj)
			g.Expect(err).NotTo(HaveOccurred())

			if convertible, isConvertible := tt.obj.(conversionmeta.Convertible); isConvertible {
				convertible.SetSource(conversionmeta.SourceTypeMeta{APIVersion: tmpSourceVersion})
			}

			objModified := tt.modifyFunc(tt.obj)

			patch := client.MergeFrom(tt.obj)
			if c.converter.IsHub(tt.obj) {
				patch, err = MergeFrom(ctx, c, tt.obj)
				g.Expect(err).ToNot(HaveOccurred())
			}

			err = c.Patch(ctx, objModified, patch)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			gvk, err := c.internalClient.GroupVersionKindFor(tt.obj)
			g.Expect(err).NotTo(HaveOccurred())

			o, err := c.internalClient.Scheme().New(gvk)
			g.Expect(err).NotTo(HaveOccurred())

			gotObj := o.(client.Object)
			err = c.Get(ctx, client.ObjectKeyFromObject(objModified), gotObj)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(gotObj).To(Equal(objModified))
		})
	}

	t.Run("Fails with wrong patch type", func(t *testing.T) {
		g := NewWithT(t)
		ctx := t.Context()

		cc, err := NewWithConverter(
			fake.NewClientBuilder().WithScheme(scheme).Build(),
			v1alpha5Converter,
		)
		g.Expect(err).NotTo(HaveOccurred())
		c := cc.(*conversionClient)

		obj := &vmoprvhub.VirtualMachine{}
		objModified := &vmoprvhub.VirtualMachine{}

		err = c.Patch(ctx, objModified, client.MergeFrom(obj))
		g.Expect(err).To(HaveOccurred())
	})
}

func Test_conversionClient_DeleteAllOf(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		wantErr       bool
	}{
		{
			name:          "Delete non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-n",
					Namespace: "test-ns",
				},
			},
		},
		{
			name:          "Return error for hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			err = c.DeleteAllOf(ctx, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}

func Test_conversionClient_Apply(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           runtime.ApplyConfiguration
		wantErr       bool
	}{
		{
			name:          "Apply non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj:           corev1applyconfigurations.Node("test-vm").WithLabels(map[string]string{"foo": "bar"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			err = c.Apply(ctx, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err.Error()).To(ContainSubstring("invalid: fieldManager: Required value: is required for apply patch"))
		})
	}
}

func Test_conversionClient_PatchStatus(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		modifyFunc    func(client.Object) client.Object
		wantErr       bool
	}{
		{
			name:          "Patch VirtualMachine status when target version is v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Status.NodeName = "foo"
				return vm
			},
			wantErr: false,
		},
		{
			name:          "Patch VirtualMachine status when target version is v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Status.NodeName = "foo"
				return vm
			},
			wantErr: false,
		},
		{
			name:          "Patch status for non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-n",
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				n := o.(*corev1.Node)
				n.Status.DeclaredFeatures = []string{"foo"}
				return n
			},
		},
		{
			name:          "Accepts Source.APIVersion when equal to target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha5.GroupVersion.String(),
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Spec.ClassName = "another-class"
				return vm
			},
		},
		{
			name:          "Fails when Source.APIVersion different from target version",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj: &vmoprvhub.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: vmoprvhub.VirtualMachineSpec{
					ClassName: "test-class",
				},
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: vmoprv1alpha2.GroupVersion.String(),
				},
			},
			modifyFunc: func(o client.Object) client.Object {
				vm := o.(*vmoprvhub.VirtualMachine)
				vm.Spec.ClassName = "another-class"
				return vm
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&vmoprvhub.VirtualMachine{}, &vmoprv1alpha2.VirtualMachine{}, &vmoprv1alpha5.VirtualMachine{}).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			tmpSourceVersion := ""
			if convertible, isConvertible := tt.obj.(conversionmeta.Convertible); isConvertible {
				tmpSourceVersion = convertible.GetSource().APIVersion
				convertible.SetSource(conversionmeta.SourceTypeMeta{})
			}

			err = c.Create(ctx, tt.obj)
			g.Expect(err).NotTo(HaveOccurred())

			if convertible, isConvertible := tt.obj.(conversionmeta.Convertible); isConvertible {
				convertible.SetSource(conversionmeta.SourceTypeMeta{APIVersion: tmpSourceVersion})
			}

			objModified := tt.modifyFunc(tt.obj)

			patch := client.MergeFrom(tt.obj)
			if c.converter.IsHub(tt.obj) {
				patch, err = MergeFrom(ctx, c, tt.obj)
				g.Expect(err).ToNot(HaveOccurred())
			}

			err = c.Status().Patch(ctx, objModified, patch)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			gvk, err := c.internalClient.GroupVersionKindFor(tt.obj)
			g.Expect(err).NotTo(HaveOccurred())

			o, err := c.internalClient.Scheme().New(gvk)
			g.Expect(err).NotTo(HaveOccurred())

			gotObj := o.(client.Object)
			err = c.Get(ctx, client.ObjectKeyFromObject(objModified), gotObj)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(gotObj).To(Equal(objModified))
		})
	}
}

func Test_conversionClient_ApplyStatus(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           runtime.ApplyConfiguration
		wantErr       bool
	}{
		{
			name:          "Apply non hub objects",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj:           corev1applyconfigurations.Node("test-vm").WithStatus(&corev1applyconfigurations.NodeStatusApplyConfiguration{Phase: ptr.To(corev1.NodeRunning)}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			err = c.Status().Apply(ctx, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err.Error()).To(ContainSubstring("invalid: fieldManager: Required value: is required for apply patch"))
		})
	}
}

func Test_newTargetVersionObjectFor(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		wantObj       client.Object
		wantErr       bool
	}{
		{
			name:          "Create object for v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj:           &vmoprvhub.VirtualMachine{},
			wantObj:       &vmoprv1alpha2.VirtualMachine{},
		},
		{
			name:          "Create object for v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj:           &vmoprvhub.VirtualMachine{},
			wantObj:       &vmoprv1alpha5.VirtualMachine{},
		},
		{
			name:          "Fails for non hub types",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj:           &corev1.Node{},
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.obj).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			gotObj, err := c.newSpokeObjectFor(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(gotObj).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotObj).To(Equal(tt.wantObj))
		})
	}
}

func Test_newTargetVersionObjectListFor(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.ObjectList
		wantObj       client.ObjectList
		wantErr       bool
	}{
		{
			name:          "Create object list for v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj:           &vmoprvhub.VirtualMachineList{},
			wantObj:       &vmoprv1alpha2.VirtualMachineList{},
		},
		{
			name:          "Create object list for v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj:           &vmoprvhub.VirtualMachineList{},
			wantObj:       &vmoprv1alpha5.VirtualMachineList{},
		},
		{
			name:          "Fails for non hub types",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj:           &corev1.NodeList{},
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				converterForVersion(tt.targetVersion),
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			gotObj, err := c.newSpokeObjectListFor(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(gotObj).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotObj).To(Equal(tt.wantObj))
		})
	}
}

func Test_newObjectListItemFor(t *testing.T) {
	tests := []struct {
		name    string
		obj     client.ObjectList
		wantObj client.Object
	}{
		{
			name:    "Create object list item for hub VirtualMachineList",
			obj:     &vmoprvhub.VirtualMachineList{},
			wantObj: &vmoprvhub.VirtualMachine{},
		},
		{
			name:    "Create object list item for v1alpha2 VirtualMachineList",
			obj:     &vmoprv1alpha2.VirtualMachineList{},
			wantObj: &vmoprv1alpha2.VirtualMachine{},
		},
		{
			name:    "Create object list item for v1alpha5 VirtualMachineList",
			obj:     &vmoprv1alpha5.VirtualMachineList{},
			wantObj: &vmoprv1alpha5.VirtualMachine{},
		},
		{
			name:    "Create object list item for non hub types",
			obj:     &corev1.NodeList{},
			wantObj: &corev1.Node{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cc, err := NewWithConverter(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				v1alpha5Converter,
			)
			g.Expect(err).NotTo(HaveOccurred())
			c := cc.(*conversionClient)

			gotObj, err := c.newObjectListItemFor(tt.obj)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotObj).To(Equal(tt.wantObj))
		})
	}
}
