/*
Copyright 2021 The Kubernetes Authors.

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
	goctx "context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/simulator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirecord "k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

func TestReconcileNormal_WaitingForIPAddrAllocation(t *testing.T) {
	// initializing a fake server to replace the vSphere endpoint
	model := simulator.VPX()
	model.Host = 0

	simr, err := helpers.VCSimBuilder().WithModel(model).Build()
	if err != nil {
		t.Fatalf("unable to create simulator: %s", err)
	}
	defer simr.Destroy()

	vsphereCluster := &infrav1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-vsphere-cluster",
			Namespace: "test",
		},
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-cluster",
			Namespace: "test",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Name: vsphereCluster.Name,
			},
		},
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "valid-cluster",
			},
		},
	}

	vsphereMachine := &infrav1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-vm",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "valid-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine", Name: "foo"}},
		},
	}

	vSphereVM := &infrav1.VSphereVM{
		TypeMeta: metav1.TypeMeta{
			Kind: "VSphereVM",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "valid-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: infrav1.GroupVersion.String(), Kind: "VSphereMachine", Name: "foo-vm"}},
			// To make sure PatchHelper does not error out
			ResourceVersion: "1234",
		},
		Spec: infrav1.VSphereVMSpec{
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
				Server: simr.ServerURL().Host,
				Network: infrav1.NetworkSpec{
					Devices: []infrav1.NetworkDeviceSpec{
						{NetworkName: "nw-1"},
						{NetworkName: "nw-2"},
					},
				},
			},
		},
		Status: infrav1.VSphereVMStatus{},
	}

	controllerMgrContext := fake.NewControllerManagerContext(vSphereVM, vsphereMachine, machine, cluster, vsphereCluster)
	password, _ := simr.ServerURL().User.Password()
	controllerMgrContext.Password = password
	controllerMgrContext.Username = simr.ServerURL().User.Username()

	controllerContext := &context.ControllerContext{
		ControllerManagerContext: controllerMgrContext,
		Recorder:                 record.New(apirecord.NewFakeRecorder(100)),
		Logger:                   log.Log,
	}
	r := vmReconciler{ControllerContext: controllerContext}

	_, err = r.Reconcile(goctx.Background(), ctrl.Request{NamespacedName: util.ObjectKey(vSphereVM)})
	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())

	vm := &infrav1.VSphereVM{}
	vmKey := util.ObjectKey(vSphereVM)
	g.Expect(r.Client.Get(goctx.Background(), vmKey, vm)).NotTo(HaveOccurred())

	g.Expect(conditions.Has(vm, infrav1.VMProvisionedCondition)).To(BeTrue())
	vmProvisionCondition := conditions.Get(vm, infrav1.VMProvisionedCondition)
	g.Expect(vmProvisionCondition.Status).To(Equal(corev1.ConditionFalse))
	g.Expect(vmProvisionCondition.Reason).To(Equal(infrav1.WaitingForStaticIPAllocationReason))
}

func TestVmReconciler_WaitingForStaticIPAllocation(t *testing.T) {
	tests := []struct {
		name       string
		devices    []infrav1.NetworkDeviceSpec
		shouldWait bool
	}{
		{
			name:       "for one n/w device with DHCP set to true",
			devices:    []infrav1.NetworkDeviceSpec{{DHCP4: true, NetworkName: "nw-1"}},
			shouldWait: false,
		},
		{
			name: "for multiple n/w devices with DHCP set and unset",
			devices: []infrav1.NetworkDeviceSpec{
				{DHCP4: true, NetworkName: "nw-1"},
				{NetworkName: "nw-2"},
			},
			shouldWait: true,
		},
		{
			name: "for multiple n/w devices with static IP address specified",
			devices: []infrav1.NetworkDeviceSpec{
				{NetworkName: "nw-1", IPAddrs: []string{"192.168.1.2/32"}},
				{NetworkName: "nw-2"},
			},
			shouldWait: true,
		},
		{
			name: "for single n/w devices with DHCP4, DHCP6 & IP address unset",
			devices: []infrav1.NetworkDeviceSpec{
				{NetworkName: "nw-1"},
			},
			shouldWait: true,
		},
		{
			name: "for multiple n/w devices with DHCP4, DHCP6 & IP address unset",
			devices: []infrav1.NetworkDeviceSpec{
				{NetworkName: "nw-1"},
				{NetworkName: "nw-2"},
			},
			shouldWait: true,
		},
	}

	controllerCtx := fake.NewControllerContext(fake.NewControllerManagerContext())
	vmContext := fake.NewVMContext(controllerCtx)
	r := vmReconciler{controllerCtx}

	for _, tt := range tests {
		// Need to explicitly reinitialize test variable, looks odd, but needed
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			vmContext.VSphereVM.Spec.Network = infrav1.NetworkSpec{Devices: tt.devices}
			isWaiting := r.isWaitingForStaticIPAllocation(vmContext)
			g := NewWithT(t)
			g.Expect(isWaiting).To(Equal(tt.shouldWait))
		})
	}
}

func TestRetrievingVCenterCredentialsFromCluster(t *testing.T) {
	// initializing a fake server to replace the vSphere endpoint
	model := simulator.VPX()
	model.Host = 0

	simr, err := helpers.VCSimBuilder().WithModel(model).Build()
	if err != nil {
		t.Fatalf("unable to create simulator: %s", err)
	}
	defer simr.Destroy()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "creds-secret",
			Namespace: "test",
		},
		Data: map[string][]byte{
			identity.UsernameKey: []byte(simr.Username()),
			identity.PasswordKey: []byte(simr.Password()),
		},
	}

	vsphereCluster := &infrav1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-vsphere-cluster",
			Namespace: "test",
		},
		Spec: infrav1.VSphereClusterSpec{
			IdentityRef: &infrav1.VSphereIdentityReference{
				Kind: infrav1.SecretKind,
				Name: secret.Name,
			},
		},
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-cluster",
			Namespace: "test",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Name: vsphereCluster.Name,
			},
		},
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "valid-cluster",
			},
		},
	}

	vsphereMachine := &infrav1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-vm",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "valid-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine", Name: "foo"}},
		},
	}

	vSphereVM := &infrav1.VSphereVM{
		TypeMeta: metav1.TypeMeta{
			Kind: "VSphereVM",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "valid-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: infrav1.GroupVersion.String(), Kind: "VSphereMachine", Name: "foo-vm"}},
			// To make sure PatchHelper does not error out
			ResourceVersion: "1234",
		},
		Spec: infrav1.VSphereVMSpec{
			VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
				Server: simr.ServerURL().Host,
				Network: infrav1.NetworkSpec{
					Devices: []infrav1.NetworkDeviceSpec{
						{NetworkName: "nw-1"},
						{NetworkName: "nw-2"},
					},
				},
			},
		},
		Status: infrav1.VSphereVMStatus{},
	}

	controllerMgrContext := fake.NewControllerManagerContext(secret, vSphereVM, vsphereMachine, machine, cluster, vsphereCluster)

	controllerContext := &context.ControllerContext{
		ControllerManagerContext: controllerMgrContext,
		Recorder:                 record.New(apirecord.NewFakeRecorder(100)),
		Logger:                   log.Log,
	}
	r := vmReconciler{ControllerContext: controllerContext}

	_, err = r.Reconcile(goctx.Background(), ctrl.Request{NamespacedName: util.ObjectKey(vSphereVM)})
	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())

	vm := &infrav1.VSphereVM{}
	vmKey := util.ObjectKey(vSphereVM)
	g.Expect(r.Client.Get(goctx.Background(), vmKey, vm)).NotTo(HaveOccurred())
	g.Expect(conditions.Has(vm, infrav1.VCenterAvailableCondition)).To(BeTrue())
	vCenterCondition := conditions.Get(vm, infrav1.VCenterAvailableCondition)
	g.Expect(vCenterCondition.Status).To(Equal(corev1.ConditionTrue))
}
