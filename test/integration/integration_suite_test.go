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

//nolint
package integration

import (
	goctx "context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
)

const (
	loglevel                           = "5"
	waitTimeSecsForExists              = 60
	dummyVirtualMachineImageName       = "dummy-image"
	dummyDistributionVersion           = "dummy-distro.123"
	dummyImageRepository               = "vmware"
	dummyDnsVersion                    = "v1.3.1_vmware.1"
	dummyEtcdVersion                   = "v3.3.10_vmware.1"
	numControlPlaneMachines            = 1
	controlPlaneMachineClassName       = "dummy-control-plane-class"
	controlPlaneMachineStorageClass    = "dummy-control-plane-storage-class"
	controlPlaneEndPoint               = "https://dummy-lb:6443"
	numWorkerMachines                  = 1
	VirtualMachineDistributionProperty = "vmware-system.guest.kubernetes.distribution.image.version"
)

var (
	testClusterName        string
	dummyKubernetesVersion = "1.15.0+vmware.1"
	ctx                    goctx.Context
	k8sClient              dynamic.Interface
)

var (
	intervals = []interface{}{
		time.Second * time.Duration(waitTimeSecsForExists),
		time.Second * 1,
	}

	clustersResource = schema.GroupVersionResource{
		Group:    clusterv1.GroupVersion.Group,
		Version:  clusterv1.GroupVersion.Version,
		Resource: "clusters",
	}

	vsphereclustersResource = schema.GroupVersionResource{
		Group:    infrav1.GroupVersion.Group,
		Version:  infrav1.GroupVersion.Version,
		Resource: "vsphereclusters",
	}

	machinesResource = schema.GroupVersionResource{
		Group:    clusterv1.GroupVersion.Group,
		Version:  clusterv1.GroupVersion.Version,
		Resource: "machines",
	}

	machinedeploymentResource = schema.GroupVersionResource{
		Group:    clusterv1.GroupVersion.Group,
		Version:  clusterv1.GroupVersion.Version,
		Resource: "machinedeployments",
	}

	vspheremachinesResource = schema.GroupVersionResource{
		Group:    infrav1.GroupVersion.Group,
		Version:  infrav1.GroupVersion.Version,
		Resource: "vspheremachines",
	}

	vspheremachinetemplateResource = schema.GroupVersionResource{
		Group:    infrav1.GroupVersion.Group,
		Version:  infrav1.GroupVersion.Version,
		Resource: "vspheremachinetemplates",
	}

	kubeadmconfigResources = schema.GroupVersionResource{
		Group:    bootstrapv1.GroupVersion.Group,
		Version:  bootstrapv1.GroupVersion.Version,
		Resource: "kubeadmconfigs",
	}

	kubeadmconfigtemplateResource = schema.GroupVersionResource{
		Group:    bootstrapv1.GroupVersion.Group,
		Version:  bootstrapv1.GroupVersion.Version,
		Resource: "kubeadmconfigtemplates",
	}

	namespacesResource = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	configmapsResource = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	eventsResource = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}

	virtualmachinesResource = schema.GroupVersionResource{
		Group:    vmoprv1.SchemeGroupVersion.Group,
		Version:  vmoprv1.SchemeGroupVersion.Version,
		Resource: "virtualmachines",
	}

	virtualmachineimageResource = schema.GroupVersionResource{
		Group:    vmoprv1.SchemeGroupVersion.Group,
		Version:  vmoprv1.SchemeGroupVersion.Version,
		Resource: "virtualmachineimages",
	}
)

// Manifests contains the resources required to deploy a cluster with CAPV.
type Manifests struct {
	ClusterComponents *ClusterComponents

	ControlPlaneComponentsList []*ControlPlaneComponents

	WorkerComponents *WorkerComponents
}

type ClusterComponents struct {
	Cluster        *clusterv1.Cluster
	VSphereCluster *infrav1.VSphereCluster
}

// ControlPlaneComponents contains the resources required to create a control
// plane machine.
type ControlPlaneComponents struct {
	Machine        *clusterv1.Machine
	VSphereMachine *infrav1.VSphereMachine
	KubeadmConfig  *bootstrapv1.KubeadmConfig
}

// WorkerComponents contains the resources required to create a
// MachineDeployment.
type WorkerComponents struct {
	MachineDeployment      *clusterv1.MachineDeployment
	VSphereMachineTemplate *infrav1.VSphereMachineTemplate
	KubeadmConfigTemplate  *bootstrapv1.KubeadmConfigTemplate
}

func TestCAPV(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CAPV Supervisor integration tests")
}

// Test suite flags
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool
)

// Test suite global vars
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy
)

func init() {
	flag.StringVar(&configPath, "config", "", "path to the config file")
	flag.StringVar(&artifactFolder, "artifacts-folder", "", "folder where test artifacts should be stored")
	flag.BoolVar(&skipCleanup, "skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Before all ParallelNodes.
	flags := flag.NewFlagSet("flags", flag.PanicOnError)
	klog.InitFlags(flags)
	ctrl.SetLogger(klog.Background())
	err := flags.Parse([]string{"-v", loglevel})
	Expect(err).NotTo(HaveOccurred())

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder)

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	Byf("Loading the e2e test configuration from %q", configPath)
	e2eConfig, err = helpers.LoadE2EConfig(configPath)
	Expect(err).NotTo(HaveOccurred())

	Byf("Creating a clusterctl local repository into %q", artifactFolder)
	clusterctlConfigPath, err = helpers.CreateClusterctlLocalRepository(e2eConfig, filepath.Join(artifactFolder, "repository"), false)
	Expect(err).NotTo(HaveOccurred())

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy, err = helpers.SetupBootstrapCluster(e2eConfig, scheme, useExistingCluster)
	Expect(err).NotTo(HaveOccurred())

	By("Initializing the bootstrap cluster")
	helpers.InitBootstrapCluster(bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)
	ctx = goctx.Background()
	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.
	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(4))

	artifactFolder = parts[0]
	configPath = parts[1]
	clusterctlConfigPath = parts[2]
	kubeconfigPath := parts[3]

	var err error
	e2eConfig, err = helpers.LoadE2EConfig(configPath)
	Expect(err).NotTo(HaveOccurred())
	bootstrapClusterProxy = framework.NewClusterProxy("bootstrap", kubeconfigPath, initScheme())
	config := bootstrapClusterProxy.GetRESTConfig()
	k8sClient = dynamic.NewForConfigOrDie(config)
})

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
// The local clusterctl repository is preserved like everything else created into the artifact folder.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
}, func() {
	// After all ParallelNodes.

	By("Tearing down the management cluster")
	if !skipCleanup {
		helpers.TearDown(bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	_ = infrav1.AddToScheme(sc)
	return sc
}

type ImageVersion struct {
	ImageRepository string `json:"imageRepository"`
	Version         string `json:"version"`
}

type VirtualMachineDistributionSpec struct {
	Version    string       `json:"version"`
	Kubernetes ImageVersion `json:"kubernetes"`
	Etcd       ImageVersion `json:"etcd"`
	CoreDNS    ImageVersion `json:"coredns"`
}

func generateVirtualMachineImage() *vmoprv1.VirtualMachineImage {
	annotations := map[string]string{}

	spec := &VirtualMachineDistributionSpec{
		Version: dummyDistributionVersion,
		Kubernetes: ImageVersion{
			ImageRepository: dummyImageRepository,
			Version:         dummyKubernetesVersion,
		},
		Etcd: ImageVersion{
			ImageRepository: dummyImageRepository,
			Version:         dummyEtcdVersion,
		},
		CoreDNS: ImageVersion{
			ImageRepository: dummyImageRepository,
			Version:         dummyDnsVersion,
		},
	}

	rawJSON, err := json.Marshal(spec)
	Expect(err).NotTo(HaveOccurred())

	annotations[VirtualMachineDistributionProperty] = string(rawJSON)

	return &vmoprv1.VirtualMachineImage{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VirtualMachineImage",
			APIVersion: virtualmachineimageResource.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        dummyVirtualMachineImageName,
			Annotations: annotations,
		},
	}
}

func generateManifests(testNamespace string) *Manifests {
	By("Creating Cluster Components")
	clusterComponents := createClusterComponents(testNamespace)

	By("Creating ControlPlane Components List")
	controlPlaneComponentsList := createControlPlaneComponentsList(testNamespace)

	By("Creating Worker Components")
	workerComponents := createWorkerComponents(testNamespace)

	return &Manifests{
		ClusterComponents: clusterComponents,

		ControlPlaneComponentsList: controlPlaneComponentsList,

		WorkerComponents: workerComponents,
	}
}

func createClusterComponents(testNamespace string) *ClusterComponents {
	By("Creating a VSphereCluster")
	vsphereCluster := infrav1.VSphereCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "VSphereCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testNamespace,
		},
	}

	By("Creating a Cluster")
	cluster := clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testNamespace,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"100.64.0.0/13"},
				},
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{"100.96.0.0/11"},
				},
				ServiceDomain: "cluster.local",
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "VSphereCluster",
				Name:       vsphereCluster.Name,
				Namespace:  vsphereCluster.Namespace,
			},
		},
	}

	return &ClusterComponents{
		Cluster:        &cluster,
		VSphereCluster: &vsphereCluster,
	}
}

func createControlPlaneComponentsList(testNamespace string) []*ControlPlaneComponents {

	cpMachineNameFmt := "%s-control-plane-%d"
	var controlPlaneComponentsList []*ControlPlaneComponents

	for i := 0; i < numControlPlaneMachines; i++ {
		var controlPlaneComponents ControlPlaneComponents

		By("Creating a VSphereMachine")
		vsphereMachine := infrav1.VSphereMachine{
			TypeMeta: metav1.TypeMeta{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "VSphereMachine",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(cpMachineNameFmt, testClusterName, i),
				Namespace: testNamespace,
				Labels: map[string]string{
					clusterv1.MachineControlPlaneLabelName: "true",
					clusterv1.ClusterLabelName:             testClusterName,
				},
			},
			Spec: infrav1.VSphereMachineSpec{
				ImageName:    dummyVirtualMachineImageName,
				ClassName:    controlPlaneMachineClassName,
				StorageClass: controlPlaneMachineStorageClass,
			},
		}
		controlPlaneComponents.VSphereMachine = &vsphereMachine

		By("Creating KubeadmConfigs")

		kubeadmConfig := bootstrapv1.KubeadmConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: bootstrapv1.GroupVersion.String(),
				Kind:       "KubeadmConfig",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(cpMachineNameFmt, testClusterName, i),
				Namespace: testNamespace,
			},
			Spec: bootstrapv1.KubeadmConfigSpec{
				ClusterConfiguration: &bootstrapv1.ClusterConfiguration{
					ClusterName: testClusterName,
				},
				InitConfiguration: &bootstrapv1.InitConfiguration{},
				JoinConfiguration: &bootstrapv1.JoinConfiguration{},
			},
		}
		controlPlaneComponents.KubeadmConfig = &kubeadmConfig

		By("Creating Machines")
		machine := clusterv1.Machine{
			TypeMeta: metav1.TypeMeta{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Machine",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(cpMachineNameFmt, testClusterName, i),
				Namespace: testNamespace,
				Labels: map[string]string{
					clusterv1.MachineControlPlaneLabelName: "true",
					clusterv1.ClusterLabelName:             testClusterName,
				},
			},
			Spec: clusterv1.MachineSpec{
				ClusterName: testClusterName,
				Version:     &dummyKubernetesVersion,
				Bootstrap: clusterv1.Bootstrap{
					ConfigRef: &corev1.ObjectReference{
						APIVersion: bootstrapv1.GroupVersion.String(),
						Kind:       "KubeadmConfig",
						Name:       kubeadmConfig.Name,
						Namespace:  kubeadmConfig.Namespace,
					},
				},
				InfrastructureRef: corev1.ObjectReference{
					APIVersion: infrav1.GroupVersion.String(),
					Kind:       "VSphereMachine",
					Name:       vsphereMachine.Name,
					Namespace:  vsphereMachine.Namespace,
				},
			},
		}

		controlPlaneComponents.Machine = &machine

		// Collect controlPlaneComponents
		controlPlaneComponentsList = append(controlPlaneComponentsList, &controlPlaneComponents)
	}

	return controlPlaneComponentsList
}

func createWorkerComponents(testNamespace string) *WorkerComponents {
	workerMachineDeploymentNameFmt := "%s-workers-0"

	By("Creating VSphereMachineTemplates")
	vsphereMachineTemplate := infrav1.VSphereMachineTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "VSphereMachineTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(workerMachineDeploymentNameFmt, testClusterName),
			Namespace: testNamespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: testClusterName,
			},
		},
	}

	By("Creating a KubeadmConfigTemplate")
	kubeadmConfigTemplate := bootstrapv1.KubeadmConfigTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: bootstrapv1.GroupVersion.String(),
			Kind:       "KubeadmConfigTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(workerMachineDeploymentNameFmt, testClusterName),
			Namespace: testNamespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: testClusterName,
			},
		},
	}

	By("Creating a MachineDeployment")
	numWorker := int32(numWorkerMachines)
	machineDeployment := clusterv1.MachineDeployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(workerMachineDeploymentNameFmt, testClusterName),
			Namespace: testNamespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: testClusterName,
			},
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: testClusterName,
			Replicas:    &numWorker,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					clusterv1.ClusterLabelName: testClusterName,
				},
			},
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: clusterv1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterLabelName: testClusterName,
					},
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: testClusterName,
					Version:     &dummyKubernetesVersion,
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: bootstrapv1.GroupVersion.String(),
							Kind:       "KubeadmConfigTemplate",
							Name:       kubeadmConfigTemplate.Name,
							Namespace:  kubeadmConfigTemplate.Namespace,
						},
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "VSphereMachineTemplate",
						Name:       vsphereMachineTemplate.Name,
						Namespace:  vsphereMachineTemplate.Namespace,
					},
				},
			},
		},
	}

	return &WorkerComponents{
		MachineDeployment:      &machineDeployment,
		VSphereMachineTemplate: &vsphereMachineTemplate,
		KubeadmConfigTemplate:  &kubeadmConfigTemplate,
	}
}

func createNonNamespacedResource(resource schema.GroupVersionResource, obj runtimeObjectWithName) {
	input := toUnstructured(obj.GetName(), obj, false)
	_, err := k8sClient.Resource(resource).Create(ctx, input, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred(), "Error creating %s %s", resource, obj.GetName())
}

func createResource(resource schema.GroupVersionResource, obj runtimeObjectWithName) {
	input := toUnstructured(obj.GetName(), obj, false)
	_, err := k8sClient.Resource(resource).Namespace(obj.GetNamespace()).Create(ctx, input, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "Error creating %s %s/%s", resource, obj.GetNamespace(), obj.GetName())
}

func deleteResource(resource schema.GroupVersionResource, name, namespace string, propagationPolicy *metav1.DeletionPropagation) {
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: propagationPolicy}
	err := k8sClient.Resource(resource).Namespace(namespace).Delete(ctx, name, deleteOptions)
	Expect(err).NotTo(HaveOccurred(), "Error deleting %s %s", resource, name)
}

func deleteNonNamespacedResource(resource schema.GroupVersionResource, name string, propagationPolicy *metav1.DeletionPropagation) {
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: propagationPolicy}
	err := k8sClient.Resource(resource).Delete(ctx, name, deleteOptions)
	Expect(err).NotTo(HaveOccurred(), "Error deleting %s %s", resource, name)
}

func getResource(resource schema.GroupVersionResource, name, namespace string, obj runtime.Object) {
	output, err := k8sClient.Resource(resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "Error getting %s %s", resource, name)
	toStructured(name, obj, output)
}

func updateResourceStatus(resource schema.GroupVersionResource, obj runtimeObjectWithName) {
	input := toUnstructured(obj.GetName(), obj, true)
	_, err := k8sClient.Resource(resource).Namespace(obj.GetNamespace()).UpdateStatus(ctx, input, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred(), "Error updating status of %s %s/%s", resource, obj.GetNamespace(), obj.GetName())
}

func assertEventuallyExists(resource schema.GroupVersionResource, name, ns string, ownerRef *metav1.OwnerReference) *unstructuredv1.Unstructured {
	var obj *unstructuredv1.Unstructured
	EventuallyWithOffset(1, func() (bool, error) {
		output, err := k8sClient.Resource(resource).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if ownerRef != nil {
			foundOwnerRef := false
			for _, ref := range output.GetOwnerReferences() {
				if ref.APIVersion != ownerRef.APIVersion {
					continue
				}
				if ref.Kind != ownerRef.Kind {
					continue
				}
				if ref.Name != ownerRef.Name {
					continue
				}
				if ref.UID != ownerRef.UID {
					continue
				}
				if ref.Controller != nil || ownerRef.Controller != nil {
					if ref.Controller == nil && ownerRef.Controller != nil {
						continue
					} else if ref.Controller != nil && ownerRef.Controller == nil {
						continue
					} else if *ref.Controller != *ownerRef.Controller {
						continue
					}
				}
				if ref.BlockOwnerDeletion != nil || ownerRef.BlockOwnerDeletion != nil {
					if ref.BlockOwnerDeletion == nil && ownerRef.BlockOwnerDeletion != nil {
						continue
					} else if ref.BlockOwnerDeletion != nil && ownerRef.BlockOwnerDeletion == nil {
						continue
					} else if *ref.BlockOwnerDeletion != *ownerRef.BlockOwnerDeletion {
						continue
					}
				}
				foundOwnerRef = true
				break
			}
			if !foundOwnerRef {
				return false, errors.Errorf(
					"Unable to find expected OwnerRef %+v for %s %s/%s: %+v",
					*ownerRef,
					output.GroupVersionKind(),
					output.GetNamespace(),
					output.GetName(),
					output.GetOwnerReferences())
			}
		}
		obj = output
		return true, nil
	}, intervals...).Should(BeTrue(), "should exist %s %s", resource, name)
	return obj
}

func assertEventuallyDoesNotExist(resource schema.GroupVersionResource, name, ns string) {
	EventuallyWithOffset(1, func() (bool, error) {
		_, err := k8sClient.Resource(resource).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}, intervals...).Should(BeTrue(), "should not exist %s %s", resource, name)
}

func assertVirtualMachineState(machine *clusterv1.Machine, vm *vmoprv1.VirtualMachine) {
	Expect(vm.Name).Should(Equal(machine.Name))
	Expect(vm.Namespace).Should(Equal(machine.Namespace))
	Expect(vm.Spec.ImageName).ShouldNot(BeEmpty())
	Expect(machine.Spec.Version).ShouldNot(BeNil(), "Error accessing nil Spec.Version for machine %s", machine.Name)
	Expect(vm.Spec.VmMetadata).NotTo(BeNil())
	Expect(vm.Spec.VmMetadata.Transport).To(Equal(vmoprv1.VirtualMachineMetadataCloudInitTransport))
	Expect(vm.Spec.VmMetadata.ConfigMapName).ToNot(BeNil())
}

// assertClusterEventuallyGetsControlPlaneEndpoint ensures that the cluster
// receives a control plane endpoint that matches the expected IP address
func assertClusterEventuallyGetsControlPlaneEndpoint(clusterName, clusterNs string, ipAddress string) {
	EventuallyWithOffset(1, func() bool {
		vsphereCluster := &infrav1.VSphereCluster{}
		getResource(vsphereclustersResource, clusterName, clusterNs, vsphereCluster)
		// If the control plane endpoint is undefined, return false
		if vsphereCluster.Spec.ControlPlaneEndpoint.IsZero() {
			return false
		}

		return vsphereCluster.Spec.ControlPlaneEndpoint.Host == ipAddress
	}, intervals...).Should(BeTrue(), "Expected ControlPlaneEndpoint was not set")
}

func toUnstructured(name string, obj runtime.Object, preserveStatus bool) *unstructuredv1.Unstructured {
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if !preserveStatus {
		delete(data, "status")
	}
	Expect(err).NotTo(
		HaveOccurred(),
		"Error getting unstructured data for %s: %q",
		obj.GetObjectKind().GroupVersionKind(), name)
	return &unstructuredv1.Unstructured{Object: data}
}

func toStructured(name string, dst runtime.Object, src *unstructuredv1.Unstructured) {
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(src.UnstructuredContent(), dst)
	Expect(err).NotTo(
		HaveOccurred(),
		"Error getting structured object for %s: %q",
		src.GroupVersionKind(), name)
}

type canBeReferenced interface {
	GetUID() types.UID
	GetName() string
	GroupVersionKind() schema.GroupVersionKind
}

type runtimeObjectWithName interface {
	canBeReferenced
	runtime.Object
	GetNamespace() string
}

func toOwnerRef(obj canBeReferenced) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: obj.GroupVersionKind().GroupVersion().String(),
		Kind:       obj.GroupVersionKind().Kind,
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
	}
}

func toControllerOwnerRef(obj canBeReferenced) *metav1.OwnerReference {
	ptrBool := true
	return &metav1.OwnerReference{
		APIVersion:         obj.GroupVersionKind().GroupVersion().String(),
		Kind:               obj.GroupVersionKind().Kind,
		Name:               obj.GetName(),
		UID:                obj.GetUID(),
		Controller:         &ptrBool,
		BlockOwnerDeletion: &ptrBool,
	}
}

func setIPAddressOnMachine(machineName, machineNs, ipAddress string) {
	vsphereMachine := &infrav1.VSphereMachine{}
	getResource(vspheremachinesResource, machineName, machineNs, vsphereMachine)
	vsphereMachine.Status.IPAddr = ipAddress
	updateResourceStatus(vspheremachinesResource, vsphereMachine)
}

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}
