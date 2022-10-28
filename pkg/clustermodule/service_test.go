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
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
)

func TestService_Create(t *testing.T) {
	svc := NewService()

	t.Run("creation is skipped", func(t *testing.T) {
		t.Run("when wrapper points to template != VSphereMachineTemplate", func(t *testing.T) {
			md := machineDeployment("md", fake.Namespace, fake.Clusterv1a2Name)
			md.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
				Kind:      "NonVSphereMachineTemplate",
				Namespace: fake.Namespace,
				Name:      "blah",
			}

			g := gomega.NewWithT(t)
			controllerCtx := fake.NewControllerContext(fake.NewControllerManagerContext(md))
			ctx := fake.NewClusterContext(controllerCtx)

			moduleUUID, err := svc.Create(ctx, mdWrapper{md})
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(moduleUUID).To(gomega.BeEmpty())
		})

		t.Run("when template uses a different vCenter URL", func(t *testing.T) {
			md := machineDeployment("md", fake.Namespace, fake.Clusterv1a2Name)
			md.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
				Kind:      "VSphereMachineTemplate",
				Namespace: fake.Namespace,
				Name:      "blah-template",
			}

			machineTemplate := &infrav1.VSphereMachineTemplate{
				TypeMeta: metav1.TypeMeta{Kind: "VSphereMachineTemplate"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "blah-template",
					Namespace: fake.Namespace,
				},
				Spec: infrav1.VSphereMachineTemplateSpec{
					Template: infrav1.VSphereMachineTemplateResource{Spec: infrav1.VSphereMachineSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{Server: fmt.Sprintf("not.%s", fake.VCenterURL)},
					}},
				},
			}

			g := gomega.NewWithT(t)
			controllerCtx := fake.NewControllerContext(fake.NewControllerManagerContext(md, machineTemplate))
			ctx := fake.NewClusterContext(controllerCtx)

			moduleUUID, err := svc.Create(ctx, mdWrapper{md})
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(moduleUUID).To(gomega.BeEmpty())
		})
	})
}

func machineDeployment(name, namespace, cluster string) *clusterv1.MachineDeployment {
	return &clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterLabelName: cluster},
		},
	}
}
