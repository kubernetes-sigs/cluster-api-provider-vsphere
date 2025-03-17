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

package test

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/controllers"
	"sigs.k8s.io/cluster-api-provider-vsphere/controllers/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	vmwarewebhooks "sigs.k8s.io/cluster-api-provider-vsphere/internal/webhooks/vmware"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/constants"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

const (
	defaultNamespace    = "default"
	useLoadBalancer     = true
	dontUseLoadBalancer = false
)

// newInfraCluster returns an Infra cluster with the same name as the target
// cluster.
func newInfraCluster(namespace string, cluster *clusterv1.Cluster) client.Object {
	return &vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: namespace,
		},
	}
}

// newAnonInfraCluster returns an Infra cluster with a generated name.
func newAnonInfraCluster(namespace string) client.Object {
	return &vmwarev1.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Finalizers: []string{
				vmwarev1.ClusterFinalizer,
			},
			GenerateName: "test-",
			Namespace:    namespace,
		},
	}
}

// newInfraMachine creates an Infra machine with the same name as the target
// machine.
func newInfraMachine(namespace string, machine *clusterv1.Machine) client.Object {
	return &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machine.Name,
			Namespace: namespace,
		},
	}
}

// newInfraMachine creates an Infra machine with a generated name.
func newAnonInfraMachine(namespace string) client.Object {
	return &vmwarev1.VSphereMachine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    namespace,
		},
	}
}

func deployNamespace(k8sClient client.Client) *corev1.Namespace {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-ns-",
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())

	namespaceKey := client.ObjectKey{Name: ns.Name}
	Eventually(func() error {
		return k8sClient.Get(ctx, namespaceKey, ns)
	}, time.Second*30).Should(Succeed())

	return ns
}

func dropNamespace(namespace *corev1.Namespace, k8sClient client.Client) {
	Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
}

// Creates and deploys a Cluster and VSphereCluster in order. Function does not
// block on VSphereCluster creation.
func deployCluster(namespace string, k8sClient client.Client) (client.ObjectKey, *clusterv1.Cluster, client.Object) {
	// A finalizer is added to prevent it from being deleted until its
	// dependents are removed.
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    namespace,
			Finalizers:   []string{"test"},
		},
		Spec: clusterv1.ClusterSpec{},
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

	clusterKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}
	Eventually(func() error {
		return k8sClient.Get(ctx, clusterKey, cluster)
	}, time.Second*30).Should(Succeed())

	By("Create the infrastructure cluster and wait for it to have a finalizer")
	infraCluster := newInfraCluster(namespace, cluster)
	infraCluster.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       cluster.Name,
			UID:        cluster.UID,
		},
	})
	Expect(k8sClient.Create(ctx, infraCluster)).To(Succeed())

	return clusterKey, cluster, infraCluster
}

// Creates and deploys a CAPI Machine. Function does not block on Machine
// creation.
func deployCAPIMachine(namespace string, cluster *clusterv1.Cluster, k8sClient client.Client) (client.ObjectKey, *clusterv1.Machine) {
	// A finalizer is added to prevent it from being deleted until its
	// dependents are removed.
	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
			Namespace:    namespace,
			Finalizers:   []string{"test"},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:         cluster.Name,
				clusterv1.MachineControlPlaneLabel: "",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: cluster.Name,
		},
	}
	Expect(k8sClient.Create(ctx, machine)).To(Succeed())
	machineKey := client.ObjectKey{Namespace: machine.Namespace, Name: machine.Name}
	return machineKey, machine
}

// Creates and deploys a VSphereMachine. Function does not block on Machine
// creation.
func deployInfraMachine(namespace string, machine *clusterv1.Machine, finalizers []string, k8sClient client.Client) (client.ObjectKey, client.Object) {
	infraMachine := newInfraMachine(namespace, machine)
	infraMachine.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Machine",
			Name:       machine.Name,
			UID:        machine.UID,
		},
	})
	infraMachine.SetFinalizers(finalizers)
	Expect(k8sClient.Create(ctx, infraMachine)).To(Succeed())
	infraMachineKey := client.ObjectKey{Namespace: infraMachine.GetNamespace(), Name: infraMachine.GetName()}
	return infraMachineKey, infraMachine
}

// Updates the InfrastructureRef of a CAPI Cluster to a VSphereCluster. Function
// does not block on update success.
func updateClusterInfraRef(cluster *clusterv1.Cluster, infraCluster client.Object, k8sClient client.Client) {
	cluster.Spec.InfrastructureRef = &corev1.ObjectReference{
		APIVersion: infraCluster.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       infraCluster.GetObjectKind().GroupVersionKind().Kind,
		Name:       infraCluster.GetName(),
	}
	Expect(k8sClient.Update(ctx, cluster)).To(Succeed())
}

func getManager(cfg *rest.Config, networkProvider string, withWebhooks bool) manager.Manager {
	localScheme := runtime.NewScheme()
	Expect(scheme.AddToScheme(localScheme)).To(Succeed())

	opts := manager.Options{
		Options: ctrlmgr.Options{
			Controller: config.Controller{
				UsePriorityQueue: ptr.To[bool](feature.Gates.Enabled(feature.PriorityQueue)),
			},
			Scheme: localScheme,
			NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
				syncPeriod := 1 * time.Second
				opts.SyncPeriod = &syncPeriod
				return cache.New(config, opts)
			},
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
		},
		KubeConfig:      cfg,
		NetworkProvider: networkProvider,
	}

	if withWebhooks {
		opts.WebhookServer = webhook.NewServer(
			webhook.Options{
				Port:    testEnv.WebhookInstallOptions.LocalServingPort,
				CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
				Host:    "0.0.0.0",
			},
		)
	}

	controllerOpts := controller.Options{MaxConcurrentReconciles: 10, SkipNameValidation: ptr.To(true)}

	opts.AddToManager = func(ctx context.Context, controllerCtx *capvcontext.ControllerManagerContext, mgr ctrlmgr.Manager) error {
		if err := controllers.AddClusterControllerToManager(ctx, controllerCtx, mgr, true, controllerOpts); err != nil {
			return err
		}

		if withWebhooks {
			if err := (&vmwarewebhooks.VSphereMachineTemplateWebhook{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&vmwarewebhooks.VSphereMachineWebhook{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
		}
		if err := vmware.AddVSphereMachineTemplateControllerToManager(ctx, controllerCtx, mgr, controllerOpts); err != nil {
			return err
		}

		return controllers.AddMachineControllerToManager(ctx, controllerCtx, mgr, true, controllerOpts)
	}

	mgr, err := manager.New(ctx, opts)
	Expect(err).NotTo(HaveOccurred())
	return mgr
}

func initManagerAndBuildClient(networkProvider string, withWebhooks bool) (client.Client, context.CancelFunc) {
	By("setting up a new manager")
	mgr := getManager(restConfig, networkProvider, withWebhooks)
	k8sClient := mgr.GetClient()

	By("starting the manager")
	managerCtx, managerCancel := context.WithCancel(ctx)

	go func() {
		managerRuntimeError := mgr.Start(managerCtx)
		if managerRuntimeError != nil {
			_, _ = fmt.Fprintln(GinkgoWriter, "Manager failed at runtime")
		}

		if withWebhooks {
			// wait for webhook port to be open prior to running tests
			waitForWebhooks()
		}
	}()

	return k8sClient, managerCancel
}

func waitForWebhooks() {
	port := testEnv.WebhookInstallOptions.LocalServingPort

	klog.Infof("Waiting for webhook port %d to be open prior to running tests", port)
	timeout := 1 * time.Second
	for {
		time.Sleep(1 * time.Second)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), timeout)
		if err != nil {
			klog.Infof("Webhook port is not ready, will retry in %v: %s", timeout, err)
			continue
		}
		if err := conn.Close(); err != nil {
			klog.Info("Connection to webhook port could not be closed. Continuing with tests...")
		}
		klog.Info("Webhook port is now open. Continuing with tests...")
		return
	}
}

func prepareClient(isLoadBalanced bool) (cli client.Client, cancelation context.CancelFunc) {
	networkProvider := ""
	if isLoadBalanced {
		networkProvider = manager.DummyLBNetworkProvider
	}

	cli, cancelation = initManagerAndBuildClient(networkProvider, false)
	return
}

// Cache the type names of the infrastructure cluster and machine.
var (
	infraClusterTypeName = reflect.TypeOf(newAnonInfraCluster(defaultNamespace)).Elem().Name()
	infraMachineTypeName = reflect.TypeOf(newAnonInfraMachine(defaultNamespace)).Elem().Name()
)

var _ = Describe("Conformance tests", func() {
	var (
		k8sClient     client.Client
		managerCancel context.CancelFunc
		key           *client.ObjectKey
		obj           *client.Object
	)

	// assertObjEventuallyExists is used to assert that eventually obj can be
	// retrieved from the API server.
	assertObjEventuallyExists := func() {
		EventuallyWithOffset(1, func() error {
			return k8sClient.Get(ctx, *key, *obj)
		}, time.Second*30).Should(Succeed())
	}

	JustAfterEach(func() {
		Expect(k8sClient.Delete(ctx, *obj)).To(Succeed())
	})

	AfterEach(func() {
		k8sClient = nil
		obj = nil
		key = nil
	})

	DescribeTable("Check infra cluster spec conformance",
		func(objectGenerator func(string) client.Object) {
			k8sClient, managerCancel = prepareClient(false)
			defer managerCancel()

			ns := deployNamespace(k8sClient)
			defer dropNamespace(ns, k8sClient)

			targetObject := objectGenerator(ns.Name)

			Expect(k8sClient.Create(ctx, targetObject)).To(Succeed())
			obj = &targetObject
			key = &client.ObjectKey{
				Namespace: targetObject.GetNamespace(),
				Name:      targetObject.GetName(),
			}

			assertObjEventuallyExists()
		},
		Entry("For infra-cluster "+infraClusterTypeName, newAnonInfraCluster),
		Entry("For infra-machine "+infraMachineTypeName, newAnonInfraMachine),
	)

})

var _ = Describe("Reconciliation tests", func() {
	var (
		k8sClient     client.Client
		managerCancel context.CancelFunc
	)

	// assertEventuallyFinalizers is used to assert an object eventually has one
	// or more finalizers.
	assertEventuallyFinalizers := func(key client.ObjectKey, obj client.Object) {
		EventuallyWithOffset(1, func() (int, error) {
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				return 0, err
			}
			return len(obj.GetFinalizers()), nil
		}, time.Second*30).Should(BeNumerically(">", 0))
	}

	assertEventuallyVMStatus := func(key client.ObjectKey, obj client.Object, expectedState vmwarev1.VirtualMachineState) {
		EventuallyWithOffset(1, func() (vmwarev1.VirtualMachineState, error) {
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				return "", err
			}
			vSphereMachine := obj.(*vmwarev1.VSphereMachine)
			return vSphereMachine.Status.VMStatus, nil
		}, time.Second*30).Should(Equal(expectedState))
	}

	assertEventuallyControlPlaneEndpoint := func(key client.ObjectKey, obj client.Object, expectedIP string) {
		EventuallyWithOffset(1, func() (string, error) {
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				return "", err
			}
			vsphereCluster := obj.(*vmwarev1.VSphereCluster)
			return vsphereCluster.Spec.ControlPlaneEndpoint.Host, nil
		}, time.Second*30).Should(Equal(expectedIP))
	}

	deleteAndWait := func(key client.ObjectKey, obj client.Object, removeFinalizers bool) {
		// Delete the object.
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())

		// Issues updates until the patch to remove the finalizers is
		// successful.
		if removeFinalizers {
			EventuallyWithOffset(1, func() error {
				if err := k8sClient.Get(ctx, key, obj); err != nil {
					return err
				}
				obj.SetFinalizers([]string{})
				return k8sClient.Update(ctx, obj)
			}, time.Second*30).Should(Succeed())
		}

		// Wait for the object to no longer be available.
		EventuallyWithOffset(1, func() error {
			return k8sClient.Get(ctx, key, obj)
		}, time.Second*30).ShouldNot(Succeed())
	}

	AfterEach(func() {
		k8sClient = nil
		managerCancel = nil
	})

	DescribeTable("Infrastructure resources should have finalizers after reconciliation",
		func(isLB bool) {
			k8sClient, managerCancel = prepareClient(isLB)
			defer managerCancel()

			By("Create target namespace")
			ns := deployNamespace(k8sClient)
			defer dropNamespace(ns, k8sClient)

			By("Create the CAPI Cluster and wait for it to exist")
			clusterKey, cluster, infraCluster := deployCluster(ns.Name, k8sClient)

			// Assert that eventually the infrastructure cluster will have a
			// finalizer.
			infraClusterKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}
			assertEventuallyFinalizers(infraClusterKey, infraCluster)

			By("Update the CAPI Cluster's InfrastructureRef")
			updateClusterInfraRef(cluster, infraCluster, k8sClient)

			By("Expect a ResourcePolicy to exist")
			rpKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}
			resourcePolicy := &vmoprv1.VirtualMachineSetResourcePolicy{}
			Eventually(func() error {
				return k8sClient.Get(ctx, rpKey, resourcePolicy)
			}, time.Second*30).Should(Succeed())
			Expect(len(resourcePolicy.Spec.ClusterModuleGroups)).To(BeEquivalentTo(2))

			By("Create the CAPI Machine and wait for it to exist")
			machineKey, machine := deployCAPIMachine(ns.Name, cluster, k8sClient)
			Eventually(func() error {
				return k8sClient.Get(ctx, machineKey, machine)
			}, time.Second*30).Should(Succeed())

			By("Create the infrastructure machine and wait for it to have a finalizer")
			infraMachineKey, infraMachine := deployInfraMachine(ns.Name, machine, nil, k8sClient)
			assertEventuallyFinalizers(infraMachineKey, infraMachine)

			// Delete the CAPI Cluster. To simulate the CAPI components we must:
			//
			// 1. Delete a resource.
			// 2. Remove its finalizers (if its a CAPI object).
			// 3. Update the resource.
			// 4. Wait for the resource to be deleted.
			By("Delete the infrastructure machine and wait for it to be removed")
			deleteAndWait(infraMachineKey, infraMachine, false)

			By("Delete the CAPI machine and wait for it to be removed")
			deleteAndWait(machineKey, machine, true)

			By("Delete the infrastructure cluster and wait for it to be removed")
			deleteAndWait(infraClusterKey, infraCluster, false)

			By("Delete the CAPI cluster and wait for it to be removed")
			deleteAndWait(clusterKey, cluster, true)
		},
		Entry("With no load balancer", dontUseLoadBalancer),
		Entry("With load balancer", useLoadBalancer),
	)

	DescribeTable("VSphereClusters can be deleted without a corresponding Cluster",
		func(isLB bool) {
			k8sClient, managerCancel = prepareClient(isLB)
			defer managerCancel()

			By("Create target namespace")
			ns := deployNamespace(k8sClient)
			defer dropNamespace(ns, k8sClient)

			By("Creating an infrastructure cluster with no owner or cluster reference")
			infraCluster := newAnonInfraCluster(ns.Name)
			Expect(k8sClient.Create(ctx, infraCluster)).To(Succeed())
			infraClusterKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}

			By("Waiting until the controller sets the finalizer")
			assertEventuallyFinalizers(infraClusterKey, infraCluster)

			By("Deleting the infrastructure cluster and waiting for it to be removed")
			deleteAndWait(infraClusterKey, infraCluster, false)
		},
		Entry("With no load balancer", dontUseLoadBalancer),
		Entry("With load balancer", useLoadBalancer),
	)

	DescribeTable("Create and Delete a VSphereMachine with a Machine but without a Cluster",
		func(isLB bool) {
			k8sClient, managerCancel = prepareClient(isLB)
			defer managerCancel()

			By("Create target namespace")
			ns := deployNamespace(k8sClient)
			defer dropNamespace(ns, k8sClient)

			By("Create the CAPI Machine and wait for it to exist")
			// A finalizer is added to prevent it from being deleted until its
			// dependents are removed.
			machine := &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    ns.Name,
					Finalizers:   []string{"test"},
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: "crud",
				},
			}
			Expect(k8sClient.Create(ctx, machine)).To(Succeed())
			machineKey := client.ObjectKey{Namespace: machine.Namespace, Name: machine.Name}
			Eventually(func() error {
				return k8sClient.Get(ctx, machineKey, machine)
			}, time.Second*30).Should(Succeed())

			By("Create the infrastructure machine and set a finalizer")
			infraMachineKey, infraMachine := deployInfraMachine(ns.Name, machine, []string{infrav1.MachineFinalizer}, k8sClient)
			Eventually(func() error {
				return k8sClient.Get(ctx, infraMachineKey, infraMachine)
			}, time.Second*30).Should(Succeed())

			By("Delete the InfraMachine and wait for it to be removed")
			deleteAndWait(infraMachineKey, infraMachine, false)
		},
		Entry("With no load balancer", dontUseLoadBalancer),
		Entry("With load balancer", useLoadBalancer),
	)

	DescribeTable("A VM gets properly reconciled for a Machine and reflects appropriate VM status",
		func(isLB bool) {
			k8sClient, managerCancel = prepareClient(isLB)
			defer managerCancel()

			By("Create target namespace")
			ns := deployNamespace(k8sClient)
			defer dropNamespace(ns, k8sClient)

			By("Create the CAPI Cluster and wait for it to exist")
			_, cluster, infraCluster := deployCluster(ns.Name, k8sClient)
			updateClusterInfraRef(cluster, infraCluster, k8sClient)
			infraClusterKey := client.ObjectKey{Namespace: infraCluster.GetNamespace(), Name: infraCluster.GetName()}
			Eventually(func() error {
				return k8sClient.Get(ctx, infraClusterKey, infraCluster)
			}, time.Second*30).Should(Succeed())
			updateClusterInfraRef(cluster, infraCluster, k8sClient)

			By("Create the CAPI Machine and wait for it to exist")
			machineKey, machine := deployCAPIMachine(ns.Name, cluster, k8sClient)
			Eventually(func() error {
				return k8sClient.Get(ctx, machineKey, machine)
			}, time.Second*30).Should(Succeed())

			By("Create the infrastructure machine and wait for it to exist")
			infraMachineKey, infraMachine := deployInfraMachine(ns.Name, machine, nil, k8sClient)
			Eventually(func() error {
				return k8sClient.Get(ctx, infraMachineKey, infraMachine)
			}, time.Second*30).Should(Succeed())

			By("Add bootstrap data to the machine")
			data := "test-bootstrap-data"
			version := "test-version"
			secretName := machine.GetName() + "-data"
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: machine.GetNamespace(),
				},
				Data: map[string][]byte{
					"value": []byte(data),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			machine.Spec.Version = &version
			machine.Spec.Bootstrap.DataSecretName = &secretName
			Expect(k8sClient.Update(ctx, machine)).To(Succeed())

			// At this point, the reconciliation loop should create a VirtualMachine Note
			// that the reconciliation loop will continue to run while a VirtualMachine is
			// going through its various stages of initialization due to vmoperator
			// code returning reconcile errors

			By("Expect the VSphereMachine to have its Status.VMStatus initialized to a new VM")
			assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStatePending)

			By("Expect the VM to have been successfully created")
			newVM := &vmoprv1.VirtualMachine{}
			Expect(k8sClient.Get(ctx, machineKey, newVM)).Should(Succeed())

			By("Modifying the VM to simulate it having been created")
			Eventually(func() error {
				err := k8sClient.Get(ctx, machineKey, newVM)
				if err != nil {
					return err
				}
				// These two lines must be initialized as requirements of having valid Status
				newVM.Status.Volumes = []vmoprv1.VirtualMachineVolumeStatus{}
				newVM.Status.Conditions = append(newVM.Status.Conditions, metav1.Condition{
					Type:               vmoprv1.VirtualMachineConditionCreated,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second)),
					Reason:             string(metav1.ConditionTrue),
				})
				return k8sClient.Status().Update(ctx, newVM)
			}, time.Second*30).Should(Succeed())

			By("Expect the VSphereMachine VM status to reflect VM Created status")
			assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStateCreated)

			By("Modifying the VM to simulate it having been powered on")
			Eventually(func() error {
				err := k8sClient.Get(ctx, machineKey, newVM)
				if err != nil {
					return err
				}
				newVM.Status.PowerState = vmoprv1.VirtualMachinePowerStateOn
				return k8sClient.Status().Update(ctx, newVM)
			}, time.Second*30).Should(Succeed())

			By("Expect the VSphereMachine VM status to reflect VM PoweredOn status")
			assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStatePoweredOn)

			By("Modifying the VM to simulate it having been successfully booted")
			Eventually(func() error {
				err := k8sClient.Get(ctx, machineKey, newVM)
				if err != nil {
					return err
				}
				if newVM.Status.Network == nil {
					newVM.Status.Network = &vmoprv1.VirtualMachineNetworkStatus{}
				}
				newVM.Status.Network.PrimaryIP4 = "1.2.3.4"
				newVM.Status.BiosUUID = "test-bios-uuid"
				newVM.Status.Host = "some-esxi-host"
				return k8sClient.Status().Update(ctx, newVM)
			}, time.Second*30).Should(Succeed())

			By("Expect the VSphereMachine VM status to reflect VM Ready status")
			assertEventuallyVMStatus(infraMachineKey, infraMachine, vmwarev1.VirtualMachineStateReady)

			By("Expect the Machine's label to reflect the ESXi host")
			EventuallyWithOffset(1, func() (string, error) {
				if err := k8sClient.Get(ctx, machineKey, machine); err != nil {
					return "", err
				}
				v, ok := machine.GetLabels()[constants.ESXiHostInfoLabel]
				if !ok {
					return "", fmt.Errorf("expect machine to have label %s", constants.ESXiHostInfoLabel)
				}

				return v, nil
			}, time.Second*30, time.Millisecond*500).Should(Equal("some-esxi-host"))

			// In the case of a LoadBalanced endpoint, ControlPlaneEndpoint is a
			// load-balancer Testing load-balanced endpoints is done in
			// control_plane_endpoint_test.go
			if !isLB {
				By("Expect the Cluster to have the IP from the VM as an APIEndpoint")
				assertEventuallyControlPlaneEndpoint(infraClusterKey, infraCluster, newVM.Status.Network.PrimaryIP4)
			}
		},
		Entry("With no load balancer", dontUseLoadBalancer),
		Entry("With load balancer", useLoadBalancer),
	)
})
