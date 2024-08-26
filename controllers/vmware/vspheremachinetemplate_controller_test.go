/*
Copyright 2024 The Kubernetes Authors.

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

package vmware

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

func Test_vSphereMachineTemplateReconciler_Reconcile(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(vmwarev1.AddToScheme(scheme)).To(Succeed())
	g.Expect(vmoprv1.AddToScheme(scheme)).To(Succeed())

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	tests := []struct {
		name                   string
		vSphereMachineTemplate *vmwarev1.VSphereMachineTemplate
		objects                []client.Object
		wantErr                string
		wantStatus             *vmwarev1.VSphereMachineTemplateStatus
	}{
		{
			name:                   "object does not exist",
			vSphereMachineTemplate: nil,
			objects:                []client.Object{},
			wantErr:                "",
			wantStatus:             nil,
		},
		{
			name:                   "VirtualMachineClass does not exist",
			vSphereMachineTemplate: vSphereMachineTemplate(namespace.Name, "no-class", "not-existing-class", nil),
			objects:                []client.Object{},
			wantErr:                "failed to get VirtualMachineClass \"not-existing-class\" for VSphereMachineTemplate",
			wantStatus:             nil,
		},
		{
			name:                   "VirtualMachineClass does exist but has no data",
			vSphereMachineTemplate: vSphereMachineTemplate(namespace.Name, "with-class", "vm-class", nil),
			objects: []client.Object{
				virtualMachineClass(namespace.Name, "vm-class", nil),
			},
			wantErr:    "",
			wantStatus: &vmwarev1.VSphereMachineTemplateStatus{},
		},
		{
			name:                   "VirtualMachineClass does exist and has cpu and memory set",
			vSphereMachineTemplate: vSphereMachineTemplate(namespace.Name, "with-class", "vm-class", nil),
			objects: []client.Object{
				virtualMachineClass(namespace.Name, "vm-class", &vmoprv1.VirtualMachineClassHardware{Cpus: 1, Memory: quantity(1024)}),
			},
			wantErr: "",
			wantStatus: &vmwarev1.VSphereMachineTemplateStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    quantity(1),
					corev1.ResourceMemory: quantity(1024),
				},
			},
		},
		{
			name: "VirtualMachineClass got updated to new cpu and memory values",
			vSphereMachineTemplate: vSphereMachineTemplate(namespace.Name, "with-class", "vm-class", &vmwarev1.VSphereMachineTemplateStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    quantity(1),
					corev1.ResourceMemory: quantity(1024),
				},
			}),
			objects: []client.Object{
				virtualMachineClass(namespace.Name, "vm-class", &vmoprv1.VirtualMachineClassHardware{Cpus: 2, Memory: quantity(2048)}),
			},
			wantErr: "",
			wantStatus: &vmwarev1.VSphereMachineTemplateStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    quantity(2),
					corev1.ResourceMemory: quantity(2048),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClientBuilder := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(append([]client.Object{namespace}, tt.objects...)...)

			vSphereMachineTemplateName := "not-exists"
			if tt.vSphereMachineTemplate != nil {
				vSphereMachineTemplateName = tt.vSphereMachineTemplate.GetName()
				fakeClientBuilder = fakeClientBuilder.
					WithObjects(tt.vSphereMachineTemplate).
					WithStatusSubresource(tt.vSphereMachineTemplate)
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      vSphereMachineTemplateName,
					Namespace: namespace.Name,
				},
			}

			r := &vSphereMachineTemplateReconciler{
				Client: fakeClientBuilder.Build(),
			}

			_, err := r.Reconcile(ctx, req)
			if tt.wantErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}

			if tt.wantStatus != nil {
				vSphereMachineTemplate := &vmwarev1.VSphereMachineTemplate{}
				g.Expect(r.Client.Get(ctx, req.NamespacedName, vSphereMachineTemplate)).To(Succeed())
				g.Expect(vSphereMachineTemplate.Status).To(BeComparableTo(*tt.wantStatus))
			}
		})
	}
}

func vSphereMachineTemplate(namespace, name, className string, status *vmwarev1.VSphereMachineTemplateStatus) *vmwarev1.VSphereMachineTemplate {
	tpl := &vmwarev1.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: vmwarev1.VSphereMachineTemplateSpec{
			Template: vmwarev1.VSphereMachineTemplateResource{
				Spec: vmwarev1.VSphereMachineSpec{
					ClassName: className,
				},
			},
		},
	}

	if status != nil {
		tpl.Status = *status
	}

	return tpl
}

func virtualMachineClass(namespace, name string, hardware *vmoprv1.VirtualMachineClassHardware) *vmoprv1.VirtualMachineClass {
	class := &vmoprv1.VirtualMachineClass{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	if hardware != nil {
		class.Spec.Hardware = *hardware
	}

	return class
}

func quantity(i int64) resource.Quantity {
	q := resource.NewQuantity(i, resource.DecimalSI)
	// Execute q.String to populate the internal s field
	_ = q.String()
	return *q
}
