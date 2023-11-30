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
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/vmware/govmomi/simulator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirecord "k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/identity"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services"
	fake_svc "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

func TestReconcileNormal_WaitingForIPAddrAllocation(t *testing.T) {
	var (
		machine *clusterv1.Machine
		cluster *clusterv1.Cluster

		vsphereVM      *infrav1.VSphereVM
		vsphereMachine *infrav1.VSphereMachine
		vsphereCluster *infrav1.VSphereCluster

		initObjs       []client.Object
		ipAddressClaim *ipamv1.IPAddressClaim
	)

	poolAPIGroup := "some.ipam.api.group"

	// initializing a fake server to replace the vSphere endpoint
	model := simulator.VPX()
	model.Host = 0
	simr, err := vcsim.NewBuilder().WithModel(model).Build()
	if err != nil {
		t.Fatalf("unable to create simulator: %s", err)
	}
	defer simr.Destroy()

	secretCachingClient, err := client.New(testEnv.Manager.GetConfig(), client.Options{
		HTTPClient: testEnv.Manager.GetHTTPClient(),
		Cache: &client.CacheOptions{
			Reader: testEnv.Manager.GetCache(),
		},
	})
	if err != nil {
		panic("unable to create secret caching client")
	}

	tracker, err := remote.NewClusterCacheTracker(
		testEnv.Manager,
		remote.ClusterCacheTrackerOptions{
			SecretCachingClient: secretCachingClient,
			ControllerName:      "testvspherevm-manager",
		},
	)
	if err != nil {
		t.Fatalf("unable to setup ClusterCacheTracker: %v", err)
	}

	controllerOpts := controller.Options{MaxConcurrentReconciles: 10}

	if err := (&remote.ClusterCacheReconciler{
		Client:  testEnv.Manager.GetClient(),
		Tracker: tracker,
	}).SetupWithManager(ctx, testEnv.Manager, controllerOpts); err != nil {
		panic(fmt.Sprintf("unable to create ClusterCacheReconciler controller: %v", err))
	}

	create := func(netSpec infrav1.NetworkSpec) func() {
		return func() {
			vsphereCluster = &infrav1.VSphereCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-vsphere-cluster",
					Namespace: "test",
				},
			}

			cluster = &clusterv1.Cluster{
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

			machine = &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "test",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "valid-cluster",
					},
				},
			}
			initObjs = createMachineOwnerHierarchy(machine)

			vsphereMachine = &infrav1.VSphereMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-vm",
					Namespace: "test",
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "valid-cluster",
					},
					OwnerReferences: []metav1.OwnerReference{{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine", Name: "foo"}},
				},
			}

			vsphereVM = &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "test",
					Finalizers: []string{
						infrav1.VMFinalizer,
					},
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "valid-cluster",
					},
					OwnerReferences: []metav1.OwnerReference{{APIVersion: infrav1.GroupVersion.String(), Kind: "VSphereMachine", Name: "foo-vm"}},
					// To make sure PatchHelper does not error out
					ResourceVersion: "1234",
				},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
						Server:     simr.ServerURL().Host,
						Datacenter: "",
						Datastore:  "",
						Network:    netSpec,
					},
				},
				Status: infrav1.VSphereVMStatus{},
			}

			ipAddressClaim = &ipamv1.IPAddressClaim{
				TypeMeta: metav1.TypeMeta{
					Kind: "IPAddressClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-0-0",
					Namespace: "test",
					Finalizers: []string{
						infrav1.IPAddressClaimFinalizer,
					},
					OwnerReferences: []metav1.OwnerReference{{APIVersion: infrav1.GroupVersion.String(), Kind: vsphereVM.Kind, Name: "foo"}},
				},
				Spec: ipamv1.IPAddressClaimSpec{
					PoolRef: corev1.TypedLocalObjectReference{
						APIGroup: &poolAPIGroup,
						Kind:     "IPAMPools",
						Name:     "my-ip-pool",
					},
				},
			}
		}
	}

	setupReconciler := func(vmService services.VirtualMachineService) vmReconciler {
		initObjs = append(initObjs, vsphereVM, vsphereMachine, machine, cluster, vsphereCluster, ipAddressClaim)
		controllerMgrContext := fake.NewControllerManagerContext(initObjs...)
		password, _ := simr.ServerURL().User.Password()
		controllerMgrContext.Password = password
		controllerMgrContext.Username = simr.ServerURL().User.Username()

		return vmReconciler{
			ControllerManagerContext:  controllerMgrContext,
			VMService:                 vmService,
			remoteClusterCacheTracker: tracker,
		}
	}

	t.Run("Waiting for static IP allocation", func(t *testing.T) {
		create(infrav1.NetworkSpec{
			Devices: []infrav1.NetworkDeviceSpec{
				{NetworkName: "nw-1"},
				{NetworkName: "nw-2"},
			},
		})()
		fakeVMSvc := new(fake_svc.VMService)
		fakeVMSvc.On("ReconcileVM", mock.Anything).Return(infrav1.VirtualMachine{
			Name:     vsphereVM.Name,
			BiosUUID: "265104de-1472-547c-b873-6dc7883fb6cb",
			State:    infrav1.VirtualMachineStatePending,
			Network:  nil,
		}, nil)
		r := setupReconciler(fakeVMSvc)
		_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: util.ObjectKey(vsphereVM)})
		g := NewWithT(t)
		g.Expect(err).NotTo(HaveOccurred())

		vm := &infrav1.VSphereVM{}
		vmKey := util.ObjectKey(vsphereVM)
		g.Expect(r.Client.Get(context.Background(), vmKey, vm)).NotTo(HaveOccurred())

		g.Expect(conditions.Has(vm, infrav1.VMProvisionedCondition)).To(BeTrue())
		vmProvisionCondition := conditions.Get(vm, infrav1.VMProvisionedCondition)
		g.Expect(vmProvisionCondition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(vmProvisionCondition.Reason).To(Equal(infrav1.WaitingForStaticIPAllocationReason))
	})

	t.Run("Waiting for IP addr allocation", func(t *testing.T) {
		create(infrav1.NetworkSpec{
			Devices: []infrav1.NetworkDeviceSpec{
				{NetworkName: "nw-1", DHCP4: true},
			},
		})()
		fakeVMSvc := new(fake_svc.VMService)
		fakeVMSvc.On("ReconcileVM", mock.Anything).Return(infrav1.VirtualMachine{
			Name:     vsphereVM.Name,
			BiosUUID: "265104de-1472-547c-b873-6dc7883fb6cb",
			State:    infrav1.VirtualMachineStateReady,
			Network: []infrav1.NetworkStatus{{
				Connected:   true,
				IPAddrs:     []string{}, // empty array to show waiting for IP address
				MACAddr:     "blah-mac",
				NetworkName: vsphereVM.Spec.Network.Devices[0].NetworkName,
			}},
		}, nil)
		r := setupReconciler(fakeVMSvc)
		_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: util.ObjectKey(vsphereVM)})
		g := NewWithT(t)
		g.Expect(err).NotTo(HaveOccurred())

		vm := &infrav1.VSphereVM{}
		vmKey := util.ObjectKey(vsphereVM)
		g.Expect(r.Client.Get(context.Background(), vmKey, vm)).NotTo(HaveOccurred())

		g.Expect(conditions.Has(vm, infrav1.VMProvisionedCondition)).To(BeTrue())
		vmProvisionCondition := conditions.Get(vm, infrav1.VMProvisionedCondition)
		g.Expect(vmProvisionCondition.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(vmProvisionCondition.Reason).To(Equal(infrav1.WaitingForIPAllocationReason))
	})

	t.Run("Deleting a VM with IPAddressClaims", func(t *testing.T) {
		create(infrav1.NetworkSpec{
			Devices: []infrav1.NetworkDeviceSpec{
				{
					NetworkName: "nw-1",
					AddressesFromPools: []corev1.TypedLocalObjectReference{
						{
							APIGroup: &poolAPIGroup,
							Kind:     "IPAMPools",
							Name:     "my-ip-pool",
						},
					},
				},
			},
		})()
		vsphereVM.ObjectMeta.Finalizers = []string{infrav1.VMFinalizer}
		vsphereVM.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: time.Now()}

		fakeVMSvc := new(fake_svc.VMService)
		fakeVMSvc.On("DestroyVM", mock.Anything).Return(reconcile.Result{}, infrav1.VirtualMachine{
			Name:     vsphereVM.Name,
			BiosUUID: "265104de-1472-547c-b873-6dc7883fb6cb",
			State:    infrav1.VirtualMachineStateNotFound,
			Network: []infrav1.NetworkStatus{{
				Connected:   true,
				IPAddrs:     []string{}, // empty array to show waiting for IP address
				MACAddr:     "blah-mac",
				NetworkName: vsphereVM.Spec.Network.Devices[0].NetworkName,
			}},
		}, nil)
		r := setupReconciler(fakeVMSvc)

		g := NewWithT(t)

		_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: util.ObjectKey(vsphereVM)})
		g.Expect(err).To(HaveOccurred())

		vm := &infrav1.VSphereVM{}
		vmKey := util.ObjectKey(vsphereVM)
		g.Expect(apierrors.IsNotFound(r.Client.Get(context.Background(), vmKey, vm))).To(BeTrue())

		claim := &ipamv1.IPAddressClaim{}
		ipacKey := util.ObjectKey(ipAddressClaim)
		g.Expect(r.Client.Get(context.Background(), ipacKey, claim)).NotTo(HaveOccurred())
		g.Expect(claim.ObjectMeta.Finalizers).NotTo(ContainElement(infrav1.IPAddressClaimFinalizer))
	})
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

	controllerManagerCtx := fake.NewControllerManagerContext()
	vmContext := fake.NewVMContext(context.Background(), controllerManagerCtx)
	r := vmReconciler{ControllerManagerContext: controllerManagerCtx}

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

	simr, err := vcsim.NewBuilder().WithModel(model).Build()
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

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "valid-cluster",
			},
		},
	}

	vsphereMachine := &infrav1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-vm",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "valid-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: clusterv1.GroupVersion.String(), Kind: "Machine", Name: "foo"}},
		},
	}

	vsphereVM := &infrav1.VSphereVM{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "test",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "valid-cluster",
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

	t.Run("Retrieve credentials from cluster", func(t *testing.T) {
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

		initObjs := createMachineOwnerHierarchy(machine)
		initObjs = append(initObjs, secret, vsphereVM, vsphereMachine, machine, cluster, vsphereCluster)
		controllerMgrContext := fake.NewControllerManagerContext(initObjs...)

		r := vmReconciler{
			Recorder:                 apirecord.NewFakeRecorder(100),
			ControllerManagerContext: controllerMgrContext,
		}

		_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: util.ObjectKey(vsphereVM)})
		g := NewWithT(t)
		g.Expect(err).NotTo(HaveOccurred())

		vm := &infrav1.VSphereVM{}
		vmKey := util.ObjectKey(vsphereVM)
		g.Expect(r.Client.Get(context.Background(), vmKey, vm)).NotTo(HaveOccurred())
		g.Expect(conditions.Has(vm, infrav1.VCenterAvailableCondition)).To(BeTrue())
		vCenterCondition := conditions.Get(vm, infrav1.VCenterAvailableCondition)
		g.Expect(vCenterCondition.Status).To(Equal(corev1.ConditionTrue))
	},
	)

	t.Run("Error if cluster infrastructureRef is nil", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-cluster",
				Namespace: "test",
			},

			// InfrastructureRef is nil so we should get an error.
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: nil,
			},
		}
		initObjs := createMachineOwnerHierarchy(machine)
		initObjs = append(initObjs, secret, vsphereVM, vsphereMachine, machine, cluster, vsphereCluster)
		controllerMgrContext := fake.NewControllerManagerContext(initObjs...)

		r := vmReconciler{
			Recorder:                 apirecord.NewFakeRecorder(100),
			ControllerManagerContext: controllerMgrContext,
		}

		_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: util.ObjectKey(vsphereVM)})
		g := NewWithT(t)
		g.Expect(err).To(HaveOccurred())
	},
	)
}

func Test_reconcile(t *testing.T) {
	ns := "test"
	vsphereCluster := &infrav1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-vsphere-cluster",
			Namespace: ns,
		},
		Spec: infrav1.VSphereClusterSpec{},
	}
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: ns,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "valid-cluster",
			},
		},
	}
	vsphereVM := &infrav1.VSphereVM{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "VSphereVM",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: ns,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "valid-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: infrav1.GroupVersion.String(), Kind: "VSphereMachine", Name: "foo-vm"}},
			Finalizers:      []string{infrav1.VMFinalizer},
		},
		Spec: infrav1.VSphereVMSpec{},
	}

	setupReconciler := func(vmService services.VirtualMachineService, initObjs ...client.Object) vmReconciler {
		return vmReconciler{
			Recorder:                 apirecord.NewFakeRecorder(100),
			ControllerManagerContext: fake.NewControllerManagerContext(initObjs...),
			VMService:                vmService,
		}
	}

	t.Run("during VM creation", func(t *testing.T) {
		initObjs := []client.Object{vsphereCluster, machine, vsphereVM}
		t.Run("when info cannot be fetched", func(t *testing.T) {
			t.Run("when anti affinity feature gate is turned off", func(t *testing.T) {
				fakeVMSvc := new(fake_svc.VMService)
				fakeVMSvc.On("ReconcileVM", mock.Anything).Return(infrav1.VirtualMachine{
					Name:     vsphereVM.Name,
					BiosUUID: "265104de-1472-547c-b873-6dc7883fb6cb",
					State:    infrav1.VirtualMachineStateReady,
				}, nil)
				r := setupReconciler(fakeVMSvc, initObjs...)
				_, err := r.reconcile(ctx, &capvcontext.VMContext{
					ControllerManagerContext: r.ControllerManagerContext,
					VSphereVM:                vsphereVM,
				}, fetchClusterModuleInput{
					VSphereCluster: vsphereCluster,
					Machine:        machine,
				})

				g := NewWithT(t)
				g.Expect(err).NotTo(HaveOccurred())
			})

			t.Run("when anti affinity feature gate is turned on", func(t *testing.T) {
				_ = feature.MutableGates.Set("NodeAntiAffinity=true")
				r := setupReconciler(new(fake_svc.VMService), initObjs...)
				_, err := r.reconcile(ctx, &capvcontext.VMContext{
					ControllerManagerContext: r.ControllerManagerContext,
					VSphereVM:                vsphereVM,
				}, fetchClusterModuleInput{
					VSphereCluster: vsphereCluster,
					Machine:        machine,
				})

				g := NewWithT(t)
				g.Expect(err).To(HaveOccurred())
			})
		})

		t.Run("when info can be fetched", func(t *testing.T) {
			objsWithHierarchy := initObjs
			objsWithHierarchy = append(objsWithHierarchy, createMachineOwnerHierarchy(machine)...)
			fakeVMSvc := new(fake_svc.VMService)
			fakeVMSvc.On("ReconcileVM", mock.Anything).Return(infrav1.VirtualMachine{
				Name:     vsphereVM.Name,
				BiosUUID: "265104de-1472-547c-b873-6dc7883fb6cb",
				State:    infrav1.VirtualMachineStateReady,
				VMRef:    "VirtualMachine:vm-129",
			}, nil)

			r := setupReconciler(fakeVMSvc, objsWithHierarchy...)
			_, err := r.reconcile(ctx, &capvcontext.VMContext{
				ControllerManagerContext: r.ControllerManagerContext,
				VSphereVM:                vsphereVM,
			}, fetchClusterModuleInput{
				VSphereCluster: vsphereCluster,
				Machine:        machine,
			})

			g := NewWithT(t)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vsphereVM.Status.VMRef).To(Equal("VirtualMachine:vm-129"))
		})
	})

	t.Run("during VM deletion", func(t *testing.T) {
		deletedVM := vsphereVM.DeepCopy()
		deletedVM.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		deletedVM.Finalizers = append(deletedVM.Finalizers, "keep-this-for-the-test")

		fakeVMSvc := new(fake_svc.VMService)
		fakeVMSvc.On("DestroyVM", mock.Anything).Return(reconcile.Result{}, infrav1.VirtualMachine{
			Name:     deletedVM.Name,
			BiosUUID: "265104de-1472-547c-b873-6dc7883fb6cb",
			State:    infrav1.VirtualMachineStateNotFound,
		}, nil)

		initObjs := []client.Object{vsphereCluster, machine, deletedVM}
		t.Run("when info can be fetched", func(t *testing.T) {
			objsWithHierarchy := initObjs
			objsWithHierarchy = append(objsWithHierarchy, createMachineOwnerHierarchy(machine)...)

			r := setupReconciler(fakeVMSvc, objsWithHierarchy...)
			_, err := r.reconcile(ctx, &capvcontext.VMContext{
				ControllerManagerContext: r.ControllerManagerContext,
				VSphereVM:                deletedVM,
			}, fetchClusterModuleInput{
				VSphereCluster: vsphereCluster,
				Machine:        machine,
			})

			g := NewWithT(t)
			g.Expect(err).NotTo(HaveOccurred())
		})

		t.Run("when info cannot be fetched", func(t *testing.T) {
			r := setupReconciler(fakeVMSvc, initObjs...)
			_, err := r.reconcile(ctx, &capvcontext.VMContext{
				ControllerManagerContext: r.ControllerManagerContext,
				VSphereVM:                deletedVM,
			}, fetchClusterModuleInput{
				VSphereCluster: vsphereCluster,
				Machine:        machine,
			})

			g := NewWithT(t)
			// Assertion to verify that cluster module info is not mandatory
			g.Expect(err).NotTo(HaveOccurred())
		})
	})
}

func createMachineOwnerHierarchy(machine *clusterv1.Machine) []client.Object {
	machine.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineSet",
			Name:       fmt.Sprintf("%s-ms", machine.Name),
		},
	}

	var (
		objs           []client.Object
		clusterName, _ = machine.Labels[clusterv1.ClusterNameLabel]
	)

	objs = append(
		objs,
		&clusterv1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-ms", machine.Name),
				Namespace: machine.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "MachineDeployment",
						Name:       fmt.Sprintf("%s-md", machine.Name),
					},
				},
			},
		},
		&clusterv1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-md", machine.Name),
				Namespace: machine.Namespace,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel: clusterName,
				},
			},
		})
	return objs
}
