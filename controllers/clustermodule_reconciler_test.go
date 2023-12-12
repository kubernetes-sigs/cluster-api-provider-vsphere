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

package controllers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/clustermodule"
	cmodfake "sigs.k8s.io/cluster-api-provider-vsphere/pkg/clustermodule/fake"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
)

func TestReconciler_Reconcile(t *testing.T) {
	kcpUUID, mdUUID := uuid.New().String(), uuid.New().String()
	kcp := controlPlane("kcp", metav1.NamespaceDefault, fake.Clusterv1a2Name)
	md := machineDeployment("md", metav1.NamespaceDefault, fake.Clusterv1a2Name)
	vCenter500err := errors.New("500 Internal Server Error")

	tests := []struct {
		name           string
		haveError      bool
		clusterModules []infrav1.ClusterModule
		beforeFn       func(object client.Object)
		setupMocks     func(*cmodfake.CMService)
		customAssert   func(*gomega.WithT, *capvcontext.ClusterContext)
	}{
		{
			name: "when cluster modules already exist",
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
				{
					ControlPlane:     false,
					TargetObjectName: "md",
					ModuleUUID:       mdUUID,
				},
			},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(true, nil)
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, mdUUID).Return(true, nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(2))
			},
		},
		{
			name:           "when no cluster modules exist",
			clusterModules: []infrav1.ClusterModule{},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(kcp)).Return(kcpUUID, nil)
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(md)).Return(mdUUID, nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(2))
				var (
					names, moduleUUIDs []string
				)
				for _, mod := range clusterCtx.VSphereCluster.Spec.ClusterModules {
					names = append(names, mod.TargetObjectName)
					moduleUUIDs = append(moduleUUIDs, mod.ModuleUUID)
				}
				g.Expect(names).To(gomega.ConsistOf("kcp", "md"))
				g.Expect(moduleUUIDs).To(gomega.ConsistOf(kcpUUID, mdUUID))
			},
		},
		{
			name: "when one cluster modules exist",
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
			},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(true, nil)
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(md)).Return(mdUUID, nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(2))
				var (
					names, moduleUUIDs []string
				)
				for _, mod := range clusterCtx.VSphereCluster.Spec.ClusterModules {
					names = append(names, mod.TargetObjectName)
					moduleUUIDs = append(moduleUUIDs, mod.ModuleUUID)
				}
				g.Expect(names).To(gomega.ConsistOf("kcp", "md"))
				g.Expect(moduleUUIDs).To(gomega.ConsistOf(kcpUUID, mdUUID))
			},
		},
		{
			name: "when cluster modules do not exist anymore",
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
				{
					ControlPlane:     false,
					TargetObjectName: "md",
					ModuleUUID:       mdUUID,
				},
			},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(false, nil)
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, mdUUID).Return(false, nil)
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(kcp)).Return(kcpUUID+"a", nil)
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(md)).Return(mdUUID+"a", nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(2))
				// Ensure the new modules exist.
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.BeElementOf(kcpUUID+"a", mdUUID+"a"))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[1].ModuleUUID).To(gomega.BeElementOf(kcpUUID+"a", mdUUID+"a"))
				// Check that condition got set.
				g.Expect(conditions.Has(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.IsTrue(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
			},
		},
		{
			name: "when cluster modules already exist but vCenter returns an error",
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
				{
					ControlPlane:     false,
					TargetObjectName: "md",
					ModuleUUID:       mdUUID,
				},
			},
			haveError: true,
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(false, vCenter500err)
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, mdUUID).Return(false, vCenter500err)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(2))
				// Ensure the old modules still exist.
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.BeElementOf(kcpUUID, mdUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[1].ModuleUUID).To(gomega.BeElementOf(kcpUUID, mdUUID))
				// Check that condition got set.
				g.Expect(conditions.Has(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.IsFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.Get(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition).Message).To(gomega.ContainSubstring(vCenter500err.Error()))
			},
		},
		{
			name: "when one cluster modules exists and for the other we get an error",
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
				{
					ControlPlane:     false,
					TargetObjectName: "md",
					ModuleUUID:       mdUUID,
				},
			},
			haveError: true,
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(true, nil)
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, mdUUID).Return(false, vCenter500err)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(2))
				// Ensure the old modules still exist.
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.BeElementOf(kcpUUID, mdUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[1].ModuleUUID).To(gomega.BeElementOf(kcpUUID, mdUUID))
				// Check that condition got set.
				g.Expect(conditions.Has(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.IsFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.Get(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition).Message).To(gomega.ContainSubstring(vCenter500err.Error()))
			},
		},
		{
			name: "when one cluster modules does not exist and for the other we get an error",
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
				{
					ControlPlane:     false,
					TargetObjectName: "md",
					ModuleUUID:       mdUUID,
				},
			},
			haveError: true,
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(false, nil)
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, mdUUID).Return(false, vCenter500err)
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(kcp)).Return(kcpUUID+"a", nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(2))
				// Ensure the errored and the new module exist.
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.BeElementOf(kcpUUID+"a", mdUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[1].ModuleUUID).To(gomega.BeElementOf(kcpUUID+"a", mdUUID))
				// Check that condition got set.
				g.Expect(conditions.Has(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.IsFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.Get(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition).Message).To(gomega.ContainSubstring(vCenter500err.Error()))
			},
		},
		{
			name:           "when cluster module creation is called for a resource pool owned by non compute cluster resource",
			clusterModules: []infrav1.ClusterModule{},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(kcp)).Return("", clustermodule.NewIncompatibleOwnerError("foo-123"))
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(md)).Return(mdUUID, nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(1))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].TargetObjectName).To(gomega.Equal("md"))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.Equal(mdUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ControlPlane).To(gomega.BeFalse())

				g.Expect(conditions.Has(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.IsFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.Get(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition).Message).To(gomega.ContainSubstring("kcp"))
			},
		},
		{
			name:           "when cluster module creation fails",
			clusterModules: []infrav1.ClusterModule{},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(kcp)).Return(kcpUUID, nil)
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(md)).Return("", errors.New("failed to reach API"))
			},
			// if cluster module creation fails for any reason apart from incompatibility, error should be returned
			haveError: true,
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(1))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].TargetObjectName).To(gomega.Equal("kcp"))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.Equal(kcpUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ControlPlane).To(gomega.BeTrue())

				g.Expect(conditions.Has(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.IsFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.Get(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition).Message).To(gomega.ContainSubstring("md"))
			},
		},
		{
			name:           "when all cluster module creations fail for a resource pool owned by non compute cluster resource",
			clusterModules: []infrav1.ClusterModule{},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(kcp)).Return("", clustermodule.NewIncompatibleOwnerError("foo-123"))
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(md)).Return("", clustermodule.NewIncompatibleOwnerError("bar-123"))
			},
			// if cluster module creation fails due to resource pool owner incompatibility, vSphereCluster object is set to Ready
			haveError: false,
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.BeEmpty())
				g.Expect(conditions.Has(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.IsFalse(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition)).To(gomega.BeTrue())
				g.Expect(conditions.Get(clusterCtx.VSphereCluster, infrav1.ClusterModulesAvailableCondition).Message).To(gomega.ContainSubstring("kcp"))
			},
		},
		{
			name:           "when some cluster module creations are skipped",
			clusterModules: []infrav1.ClusterModule{},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(kcp)).Return(kcpUUID, nil)
				// mimics cluster module creation was skipped
				svc.On("Create", mock.Anything, mock.Anything, clustermodule.NewWrapper(md)).Return("", nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(1))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].TargetObjectName).To(gomega.Equal("kcp"))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.Equal(kcpUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ControlPlane).To(gomega.BeTrue())
			},
		},
		{
			name: "when machine deployment is being deleted",
			beforeFn: func(object client.Object) {
				tym := metav1.NewTime(time.Now())
				md.ObjectMeta.DeletionTimestamp = &tym
				md.ObjectMeta.Finalizers = append(md.ObjectMeta.Finalizers, "keep-this-for-the-test")
			},
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
			},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(true, nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(1))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].TargetObjectName).To(gomega.Equal("kcp"))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.Equal(kcpUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ControlPlane).To(gomega.BeTrue())
			},
		},
		{
			name: "when machine deployment is being deleted & cluster module info is set in object",
			beforeFn: func(object client.Object) {
				tym := metav1.NewTime(time.Now())
				md.ObjectMeta.DeletionTimestamp = &tym
				md.ObjectMeta.Finalizers = append(md.ObjectMeta.Finalizers, "keep-this-for-the-test")
			},
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
				{
					ControlPlane:     false,
					TargetObjectName: "md",
					ModuleUUID:       mdUUID,
				},
			},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("DoesExist", mock.Anything, mock.Anything, mock.Anything, kcpUUID).Return(true, nil)
				svc.On("Remove", mock.Anything, mock.Anything, mdUUID).Return(nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.HaveLen(1))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].TargetObjectName).To(gomega.Equal("kcp"))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ModuleUUID).To(gomega.Equal(kcpUUID))
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules[0].ControlPlane).To(gomega.BeTrue())
			},
		},
		{
			name: "when control plane & machine deployment are being deleted & cluster module info is set in object",
			beforeFn: func(object client.Object) {
				tym := metav1.NewTime(time.Now())
				kcp.ObjectMeta.DeletionTimestamp = &tym
				kcp.ObjectMeta.Finalizers = append(kcp.ObjectMeta.Finalizers, "keep-this-for-the-test")
			},
			clusterModules: []infrav1.ClusterModule{
				{
					ControlPlane:     true,
					TargetObjectName: "kcp",
					ModuleUUID:       kcpUUID,
				},
				{
					ControlPlane:     false,
					TargetObjectName: "md",
					ModuleUUID:       mdUUID,
				},
			},
			setupMocks: func(svc *cmodfake.CMService) {
				svc.On("Remove", mock.Anything, mock.Anything, kcpUUID).Return(nil)
				svc.On("Remove", mock.Anything, mock.Anything, mdUUID).Return(nil)
			},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.BeEmpty())
			},
		},
		{
			name: "when control plane & machine deployment are being deleted & cluster module info is not set",
			beforeFn: func(object client.Object) {
				tym := metav1.NewTime(time.Now())
				kcp.ObjectMeta.DeletionTimestamp = &tym
				kcp.ObjectMeta.Finalizers = append(kcp.ObjectMeta.Finalizers, "keep-this-for-the-test")
			},
			clusterModules: []infrav1.ClusterModule{},
			customAssert: func(g *gomega.WithT, clusterCtx *capvcontext.ClusterContext) {
				g.Expect(clusterCtx.VSphereCluster.Spec.ClusterModules).To(gomega.BeEmpty())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			if tt.beforeFn != nil {
				tt.beforeFn(md)
			}
			controllerManagerContext := fake.NewControllerManagerContext(kcp, md)
			clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
			clusterCtx.VSphereCluster.Spec.ClusterModules = tt.clusterModules
			clusterCtx.VSphereCluster.Status = infrav1.VSphereClusterStatus{VCenterVersion: infrav1.NewVCenterVersion("7.0.0")}

			svc := new(cmodfake.CMService)
			if tt.setupMocks != nil {
				tt.setupMocks(svc)
			}

			r := Reconciler{
				Client:               controllerManagerContext.Client,
				ClusterModuleService: svc,
			}
			_, err := r.Reconcile(ctx, clusterCtx)
			if tt.haveError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			}
			tt.customAssert(g, clusterCtx)

			svc.AssertExpectations(t)
		})
	}
}

func TestReconciler_fetchMachineOwnerObjects(t *testing.T) {
	tests := []struct {
		name         string
		numOfMDs     int
		hasError     bool
		initObjs     []client.Object
		customAssert func(*gomega.WithT, map[string]clustermodule.Wrapper)
	}{
		{
			name: "multiple control planes",
			initObjs: []client.Object{
				controlPlane("foo-1", metav1.NamespaceDefault, fake.Clusterv1a2Name),
				controlPlane("foo-2", metav1.NamespaceDefault, fake.Clusterv1a2Name),
			},
			hasError: true,
		},
		{
			name:     "single control plane & no machine deployment",
			initObjs: []client.Object{controlPlane("foo", metav1.NamespaceDefault, fake.Clusterv1a2Name)},
			numOfMDs: 0,
		},
		{
			name: "single control plane & machine deployment",
			initObjs: []client.Object{
				controlPlane("foo", metav1.NamespaceDefault, fake.Clusterv1a2Name),
				machineDeployment("foo", metav1.NamespaceDefault, fake.Clusterv1a2Name),
				machineDeployment("foo", "bar", fake.Clusterv1a2Name),
			},
			numOfMDs: 1,
		},
		{
			name: "single control plane & multiple machine deployments",
			initObjs: []client.Object{
				controlPlane("foo", metav1.NamespaceDefault, fake.Clusterv1a2Name),
				machineDeployment("foo-1", metav1.NamespaceDefault, fake.Clusterv1a2Name),
				machineDeployment("foo-2", metav1.NamespaceDefault, fake.Clusterv1a2Name),
				machineDeployment("foo", "bar", fake.Clusterv1a2Name),
			},
			numOfMDs: 2,
			customAssert: func(g *gomega.WithT, objMap map[string]clustermodule.Wrapper) {
				g.Expect(objMap).To(gomega.HaveKey(appendKCPKey("foo")))
				g.Expect(objMap).To(gomega.HaveKey("foo-1"))
				g.Expect(objMap).To(gomega.HaveKey("foo-2"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			controllerManagerContext := fake.NewControllerManagerContext(tt.initObjs...)
			clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
			r := Reconciler{Client: controllerManagerContext.Client}
			objMap, err := r.fetchMachineOwnerObjects(ctx, clusterCtx)
			if tt.hasError {
				g.Expect(err).To(gomega.HaveOccurred())
				return
			}
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(objMap).To(gomega.HaveLen(tt.numOfMDs + 1))
			if tt.customAssert != nil {
				tt.customAssert(g, objMap)
			}
		})
	}
	t.Run("with objects marked for deletion", func(t *testing.T) {
		g := gomega.NewWithT(t)
		currTime := metav1.Now()
		mdToBeDeleted := machineDeployment("foo-1", metav1.NamespaceDefault, fake.Clusterv1a2Name)
		mdToBeDeleted.DeletionTimestamp = &currTime
		mdToBeDeleted.ObjectMeta.Finalizers = append(mdToBeDeleted.ObjectMeta.Finalizers, "keep-this-for-the-test")
		controllerManagerContext := fake.NewControllerManagerContext(
			controlPlane("foo", metav1.NamespaceDefault, fake.Clusterv1a2Name),
			machineDeployment("foo", metav1.NamespaceDefault, fake.Clusterv1a2Name),
			mdToBeDeleted,
		)
		clusterCtx := fake.NewClusterContext(ctx, controllerManagerContext)
		objMap, err := Reconciler{Client: controllerManagerContext.Client}.fetchMachineOwnerObjects(ctx, clusterCtx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(objMap).To(gomega.HaveLen(2))
	})
}

func machineDeployment(name, namespace, cluster string) *clusterv1.MachineDeployment {
	return &clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "MachineDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster},
		},
	}
}

func controlPlane(name, namespace, cluster string) *controlplanev1.KubeadmControlPlane {
	return &controlplanev1.KubeadmControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind: "KubeadmControlPlane",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster},
		},
	}
}
