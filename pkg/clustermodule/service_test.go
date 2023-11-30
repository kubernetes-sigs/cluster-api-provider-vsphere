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
	"context"
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

func TestService_Create(t *testing.T) {
	t.Run("creation is skipped", func(t *testing.T) {
		ctx := context.Background()
		t.Run("when wrapper points to template != VSphereMachineTemplate", func(t *testing.T) {
			md := machineDeployment("md", fake.Namespace, fake.Clusterv1a2Name)
			md.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
				Kind:      "NonVSphereMachineTemplate",
				Namespace: fake.Namespace,
				Name:      "blah",
			}

			g := gomega.NewWithT(t)
			controllerManagerContext := fake.NewControllerManagerContext(md)
			clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
			svc := NewService(controllerManagerContext, controllerManagerContext.Client)

			moduleUUID, err := svc.Create(ctx, clusterCtx, mdWrapper{md})
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
			controllerManagerContext := fake.NewControllerManagerContext(md, machineTemplate)
			clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
			svc := NewService(controllerManagerContext, controllerManagerContext.Client)

			moduleUUID, err := svc.Create(ctx, clusterCtx, mdWrapper{md})
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(moduleUUID).To(gomega.BeEmpty())
		})
	})

	t.Run("Create, DoesExist and Remove works", func(t *testing.T) {
		g := gomega.NewWithT(t)
		simr, err := vcsim.NewBuilder().Build()
		defer simr.Destroy()
		g.Expect(err).ToNot(gomega.HaveOccurred())
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
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						Server:       simr.ServerURL().Host,
						Datacenter:   "*",
						ResourcePool: "/DC0/host/DC0_C0/Resources",
					},
				}},
			},
		}

		controllerManagerContext := fake.NewControllerManagerContext(md, machineTemplate)
		clusterCtx := fake.NewClusterContext(context.Background(), controllerManagerContext)
		clusterCtx.VSphereCluster.Spec.Server = simr.ServerURL().Host
		controllerManagerContext.Username = simr.Username()
		controllerManagerContext.Password = simr.Password()

		svc := NewService(controllerManagerContext, controllerManagerContext.Client)
		moduleUUID, err := svc.Create(context.Background(), clusterCtx, mdWrapper{md})
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(moduleUUID).NotTo(gomega.BeEmpty())
		exists, err := svc.DoesExist(context.Background(), clusterCtx, mdWrapper{md}, moduleUUID)
		g.Expect(exists).To(gomega.BeTrue())
		g.Expect(err).NotTo(gomega.HaveOccurred())
		err = svc.Remove(context.Background(), clusterCtx, moduleUUID)
		g.Expect(err).ToNot(gomega.HaveOccurred())
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
			Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster},
		},
	}
}
