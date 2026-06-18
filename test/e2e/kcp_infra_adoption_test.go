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

package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimev1 "sigs.k8s.io/cluster-api/api/runtime/v1beta2"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/repository"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
	conversionapi "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	conversionclient "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/client"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
	vsphereframework "sigs.k8s.io/cluster-api-provider-vsphere/test/framework"
)

const (
	kcpAdoptionSpecName       = "kcp-infra-adoption" // copied from CAPI
	extensionServiceNamespace = "capv-test-extension"
	extensionServiceName      = "capv-test-extension-webhook-service"
)

var _ = Describe("When testing KCP infra adoption 1CP [vcsim] [supervisor]", func() {
	Setup(kcpAdoptionSpecName+"-1-cp", KCPInfraAdoptionTest(kcpAdoptionSpecName+"-1-cp", 1), SkipIf(testMode == GovmomiTestMode))
})

var _ = Describe("When testing KCP infra adoption 3CP [vcsim] [supervisor]", func() {
	Setup(kcpAdoptionSpecName+"-3-cp", KCPInfraAdoptionTest(kcpAdoptionSpecName+"-3-cp", 3), SkipIf(testMode == GovmomiTestMode))
})

func KCPInfraAdoptionTest(specName string, controlPlaneMachineCount int64) func(testSpecificSettingsGetter func() testSettings) {
	return func(testSpecificSettingsGetter func() testSettings) {
		// Use stable vm-operator API version or the API version defined in the VM_OPERATOR_API_VERSION env var.
		// Note: e2eConfig is null during initial ginkgo traversal.
		apiVersionVMOperator := vmoprv1alpha5.GroupVersion.Version
		if e2eConfig != nil {
			if v := e2eConfig.GetVariableOrEmpty("VM_OPERATOR_API_VERSION"); v != "" {
				apiVersionVMOperator = v
			}
		}

		if e2eConfig != nil && testTarget == VCenterTestTarget {
			Expect(e2eConfig.Variables).To(HaveKey("DOCKER_IMAGE_TAR"))
		}

		// Setup variables to be used later in the test
		var virtualMachineSetResourcePolicy *vmoprvhub.VirtualMachineSetResourcePolicy
		var virtualMachines []*vmoprvhub.VirtualMachine
		var workloadClusterProxy framework.ClusterProxy
		converter := conversionapi.DefaultConverterFor(
			schema.GroupVersion{Group: vmoprvhub.GroupVersion.Group, Version: apiVersionVMOperator},
		)

		capi_e2e.KCPInfraAdoptionSpec(ctx, func() capi_e2e.KCPInfraAdoptionSpecInput {
			return capi_e2e.KCPInfraAdoptionSpecInput{
				E2EConfig:                e2eConfig,
				ClusterctlConfigPath:     testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:    bootstrapClusterProxy,
				ArtifactFolder:           artifactFolder,
				SkipCleanup:              skipCleanup,
				Flavor:                   ptr.To(testSpecificSettingsGetter().FlavorForMode("topology-runtimesdk-self-hosted")),
				PostNamespaceCreated:     testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				ControlPlaneMachineCount: new(controlPlaneMachineCount),
				BeforeForceDelete: func(ctx context.Context, managementClusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster, objects capi_e2e.ClusterObjects) error {
					By("Saving objects required for adoption of a CAPV Cluster")

					// Create a client to get read from the management cluster.
					c, err := conversionclient.NewWithConverter(managementClusterProxy.GetClient(), converter)
					if err != nil {
						return err
					}

					// Drop owner reference from VirtualMachineSetResourcePolicy so it sticks around when force deleting the cluster and cluster objects.
					// "Orphaned" VirtualMachineSetResourcePolicy, will be re-adopted at a later stage of the test.
					virtualMachineSetResourcePolicy = &vmoprvhub.VirtualMachineSetResourcePolicy{}
					if err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}, virtualMachineSetResourcePolicy); err != nil {
						return err
					}
					original := virtualMachineSetResourcePolicy.DeepCopy()
					virtualMachineSetResourcePolicy.SetOwnerReferences([]metav1.OwnerReference{})
					patch, err := conversionclient.MergeFrom(ctx, c, original)
					if err != nil {
						return err
					}
					if err := c.Patch(ctx, virtualMachineSetResourcePolicy, patch); err != nil {
						return err
					}

					// Drop owner reference from VirtualMachine so they stick around when force deleting the cluster and cluster objects.
					// "Orphaned" VMs, are going to be re-adopted at a later stage of the test.
					for _, im := range objects.InfrastructureMachineByMachine {
						vsphereMachine := &vmwarev1.VSphereMachine{}
						if err := managementClusterProxy.GetClient().Scheme().Convert(im, vsphereMachine, nil); err != nil {
							return err
						}

						vmName, err := vmoperator.GenerateVirtualMachineName(vsphereMachine.Name, vsphereMachine.Spec.Naming)
						if err != nil {
							return err
						}

						virtualMachine := &vmoprvhub.VirtualMachine{}
						if err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: vsphereMachine.Namespace, Name: vmName}, virtualMachine); err != nil {
							return err
						}

						original := virtualMachine.DeepCopy()
						// If running with target VCenter, the test will also transform the workload cluster in a self-hoster cluster,
						// and this also requires moving the VirtualMachine to the self-hosted cluster in BeforeAdoption.
						// At this stage (BeforeForceDelete), we are pausing the VirtualMachine objects in the boostrap management cluster
						// to prevent having two controllers trying to manage them.
						if testTarget == VCenterTestTarget {
							if virtualMachine.Annotations == nil {
								virtualMachine.Annotations = map[string]string{}
							}
							virtualMachine.Annotations["vmoperator.vmware.com/paused"] = ""
						}
						virtualMachine.SetOwnerReferences([]metav1.OwnerReference{})
						patch, err := conversionclient.MergeFrom(ctx, c, original)
						if err != nil {
							return err
						}

						if err := c.Patch(ctx, virtualMachine, patch); err != nil {
							return err
						}

						virtualMachines = append(virtualMachines, virtualMachine)
					}

					// Saving a proxy to the workload cluster to be used later, when the Cluster object is deleted
					workloadClusterProxy = managementClusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name)
					return nil
				},
				BeforeAdoption: func(ctx context.Context, managementClusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster) (framework.ClusterProxy, error) {
					// If running with target different from VCenter, no-op.
					// Note: When running with target vcsim, the test will use the boostrap management cluster also for the adoption process.
					if testTarget != VCenterTestTarget {
						return managementClusterProxy, nil
					}

					// If running with target VCenter, transform the orphaned workload cluster in a CAPI management cluster,
					// so this test also validates that adoption works when the resulting cluster is self-hosted.

					By("Transforming the orphaned cluster in a self-hoster cluster")

					By(fmt.Sprintf("kubeconfig for the cluster to be adopted: %s", workloadClusterProxy.GetKubeconfigPath()))

					By("Loading required images on the VirtualMachines")
					//	Note: The manifests used to install CAPI/CAPV provider/CAPV test extension, VM Op providers are going to be used to initialize
					//	both the boostrap management cluster and the self-hosted management cluster running on the VMs being adopted.
					//
					//	In order to make this approach work, images referenced by the manifest should available in both environments, and this
					//	is complicated by the fact that we want to use images for the commit that is being tested in CI, which are not published in any image repository.
					//
					//	This is not a problem for the boostrap management cluster, because as soon as images are built locally, they are accessible
					//	from the management cluster as well. Instead, making images available on the VMs being adopted running on GVCE
					//	requires to load the images into the VMs (required images are made available in a tar file created before the test starts).
					//
					//	Please note that when running the test locally this problem might be even more complex, because boostrap management cluster
					//	and the self-hosted cluster on the VMs might be running on different platforms (e.g. arm64 for the boostrap management cluster, amd64 for the VMs on GVCE).
					//
					//	One way to get around this is the following:
					//
					//	1. Prepare the images that should be deployed on the VMs on GVCE locally, taking care of building for the target architecture.
					//
					//	ARCH=amd64 make e2e-images
					//
					//	2. Revert the change on manifests that are performed by make e2e-images.
					//
					//	3. Tag the images to match the default name existing in the manifests:
					//
					//	docker tag "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller-amd64:dev" "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller:main"
					//	docker tag "gcr.io/k8s-staging-capi-vsphere/cluster-api-vcsim-controller-amd64:dev" "gcr.io/k8s-staging-capi-vsphere/cluster-api-vcsim-controller:main"
					//	docker tag "gcr.io/k8s-staging-capi-vsphere/cluster-api-net-operator-amd64:dev" "gcr.io/k8s-staging-capi-vsphere/cluster-api-net-operator:main"
					//	docker tag "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-test-extension-amd64:dev" "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-test-extension:dev"
					//
					//	4. Create the /tmp/images/image.tar file with the tagged images (same path defined in the DOCKER_IMAGE_TAR variable in the test config file)
					//
					//	docker save \
					//		 "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-controller:main" \
					//		 "gcr.io/k8s-staging-capi-vsphere/cluster-api-vcsim-controller:main" \
					//		 "gcr.io/k8s-staging-capi-vsphere/cluster-api-net-operator:main" \
					//		 "gcr.io/k8s-staging-capi-vsphere/cluster-api-vsphere-test-extension:dev" \
					//		 > /tmp/images/image.tar
					//
					//	5. tilt-up the boostrap management cluster; tilt will take care of replacing default image names
					//     with temporary image names corresponding to images build for the boostrap management cluster architecture.
					//     Important! the tilt-settings file must use the same vm-op-version used in CI.
					//
					//	6. Run the E2E with -e2e.use-existing-cluster=true, all the other settings required for running tests on GVCE, and also
					//     the following env variable
					//
					//	Tl;DR;
					//	- we have a default, architecture agnostic, image names in the manifests.
					//	- tilt will take care of replacing default image names with temporary image names / images build for the boostrap management cluster architecture
					//	- The /tmp/images/image.tar will contain images with the same name but build for the architecture required by the VMs on GVCE
					loadImagesFunc := vsphereframework.LoadImagesFunc(ctx, e2eConfig.MustGetVariable("DOCKER_IMAGE_TAR"))
					loadImagesFunc(workloadClusterProxy)

					By("Creating test namespaces")
					// Note: Add the namespace for the Cluster and for the ClusterClass (if necessary).
					if err := createNamespaces(ctx, workloadClusterProxy, cluster); err != nil {
						return nil, err
					}

					By("Initializing the workload cluster")
					vsphereframework.InitBootstrapCluster(ctx, workloadClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)
					setupNamespaceWithVMOperatorDependenciesVCenter(workloadClusterProxy, cluster.Namespace)

					By("Deploy ExtensionConfig for the CAPV test extension")
					// Note: KCPInfraAdoptionSpec will only re-create the cluster, so it is required to re-deploy the ExtensionConfig on the self-hoster cluster,
					// using the same parameters passed to the top level test spec (extensionName, extensionServiceNamespace, extensionServiceName, namespaces).
					if err := deployExtensionConfig(ctx, specName, workloadClusterProxy, cluster.Namespace); err != nil {
						return nil, err
					}

					By("Deploying clusterclass and templates")
					// Note: KCPInfraAdoptionSpec will only re-create the cluster, so it is required to re-deploy the ClusterClass on the self-hoster cluster.
					if err := deployClusterClass(ctx, specName, workloadClusterProxy, e2eConfig, testSpecificSettingsGetter().ClusterctlConfigPath, cluster); err != nil {
						return nil, err
					}

					By("Copying cluster certificates")
					// Note: KCPInfraAdoptionSpec will only re-create the cluster, so it is required to re-deploy the cluster certificates on the self-hoster cluster.
					// Important: secrets must have the ClusterName label otherwise they are filtered out the cache.
					secrets := &corev1.SecretList{}
					if err := managementClusterProxy.GetClient().List(ctx, secrets, ctrlclient.InNamespace(cluster.Namespace), ctrlclient.MatchingLabels(map[string]string{clusterv1.ClusterNameLabel: cluster.Name})); err != nil {
						return nil, err
					}
					for _, s := range secrets.Items {
						if !secret.HasPurposeSuffix(s.Name) {
							continue
						}
						s.SetResourceVersion("")
						s.SetUID("")
						s.SetLabels(map[string]string{clusterv1.ClusterNameLabel: cluster.Name})
						s.SetAnnotations(map[string]string{})
						s.SetOwnerReferences([]metav1.OwnerReference{})
						if err := workloadClusterProxy.GetClient().Create(ctx, &s); err != nil {
							return nil, err
						}
					}

					By("Restoring objects required for adoption of a CAPV Cluster")
					// Create a client to create objects into the workload cluster.
					c, err := conversionclient.NewWithConverter(workloadClusterProxy.GetClient(), converter)
					if err != nil {
						return nil, err
					}

					// Create the VirtualMachineSetResourcePolicy.
					// Note: Cluster and vSphereCluster are recreated by KCPInfraAdoptionSpec; once vSphereCluster is created, it will adopt the VirtualMachineSetResourcePolicy.
					virtualMachineSetResourcePolicy.SetResourceVersion("")
					virtualMachineSetResourcePolicy.SetUID("")
					virtualMachineSetResourcePolicy.SetLabels(map[string]string{})
					virtualMachineSetResourcePolicy.SetAnnotations(map[string]string{})
					virtualMachineSetResourcePolicy.SetOwnerReferences([]metav1.OwnerReference{})
					virtualMachineSetResourcePolicy.SetFinalizers([]string{})
					if err := c.Create(ctx, virtualMachineSetResourcePolicy); err != nil {
						return nil, err
					}

					// Create VirtualMachines.
					// Note: Machines, KubeadmConfig and vSphereMachines are recreated by KCPInfraAdoptionSpec; once vSphereMachines are created, it will adopt the VirtualMachines.
					// Important: VirtualMachines must have the ClusterSelectorKey label otherwise they are filter out the cache. Also, vm-operator
					// assumes that a few labels or annotations are immutable, so preserving all the label/annotations except pause.
					for _, virtualMachine := range virtualMachines {
						virtualMachine := virtualMachine.DeepCopy()
						virtualMachine.SetResourceVersion("")
						virtualMachine.SetUID("")
						virtualMachine.SetOwnerReferences([]metav1.OwnerReference{})
						delete(virtualMachine.Annotations, "vmoperator.vmware.com/paused")
						if err := c.Create(ctx, virtualMachine); err != nil {
							return nil, err
						}
					}

					By("Deleting virtual machines on the bootstrap cluster")
					// Note: virtual machines on the bootstrap cluster are already paused, but remove them to avoid any risk of having two controllers managing them.
					// Note: we are not deleting the VirtualMachineSetResourcePolicy from the bootstrap cluster because it doesn't support pausing its reconciliation.
					c, err = conversionclient.NewWithConverter(managementClusterProxy.GetClient(), converter)
					if err != nil {
						return nil, err
					}

					for _, virtualMachine := range virtualMachines {
						if err := c.Delete(ctx, virtualMachine); err != nil {
							if apierrors.IsNotFound(err) {
								continue
							}
							return nil, err
						}

						original := virtualMachine.DeepCopyObject().(ctrlclient.Object)
						virtualMachine.SetFinalizers([]string{})
						patch, err := conversionclient.MergeFrom(ctx, c, original)
						if err != nil {
							return nil, err
						}

						if err := c.Patch(ctx, virtualMachine, patch); err != nil {
							if apierrors.IsNotFound(err) {
								continue
							}
							return nil, err
						}
					}

					return workloadClusterProxy, nil
				},
				// The runtime extension gets deployed to the capv-test-extension namespace and is exposed
				// by the capv-test-extension-webhook-service.
				// The below values are used when creating the cluster-wide ExtensionConfig to refer
				// the actual service.
				ExtensionConfigName:       specName,
				ExtensionServiceNamespace: "capv-test-extension",
				ExtensionServiceName:      "capv-test-extension-webhook-service",
			}
		})
	}
}

func createNamespaces(ctx context.Context, workloadClusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster.Namespace,
		},
	}
	if err := workloadClusterProxy.GetClient().Create(ctx, ns); err != nil {
		return fmt.Errorf("failed to create namespace %s", cluster.Namespace)
	}
	if cluster.Spec.Topology.ClassRef.Namespace != cluster.Namespace {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: cluster.Spec.Topology.ClassRef.Namespace,
			},
		}
		if err := workloadClusterProxy.GetClient().Create(ctx, ns); err != nil {
			return fmt.Errorf("failed to create namespace %s", cluster.Namespace)
		}
	}
	return nil
}

func deployExtensionConfig(ctx context.Context, specName string, workloadClusterProxy framework.ClusterProxy, namespaces ...string) error {
	cfg := &runtimev1.ExtensionConfig{
		ObjectMeta: metav1.ObjectMeta{
			// Note: We have to use a constant name here as we have to be able to reference it in the ClusterClass
			// when configuring external patches.
			Name: specName,
			Annotations: map[string]string{
				// Note: this assumes the test extension gets deployed in the default namespace defined in its own runtime-extensions-components.yaml
				runtimev1.InjectCAFromSecretAnnotation: fmt.Sprintf("%s/%s-cert", extensionServiceNamespace, extensionServiceName),
			},
		},
		Spec: runtimev1.ExtensionConfigSpec{
			ClientConfig: runtimev1.ClientConfig{
				Service: runtimev1.ServiceReference{
					Name: extensionServiceName,
					// Note: this assumes the test extension gets deployed in the default namespace defined in its own runtime-extensions-components.yaml
					Namespace: extensionServiceNamespace,
				},
			},
			Settings: map[string]string{
				"extensionConfigName":          specName,
				"disableInPlaceUpdates":        strconv.FormatBool(true),
				"defaultAllHandlersToBlocking": strconv.FormatBool(false),
			},
		},
	}
	if len(namespaces) > 0 {
		cfg.Spec.NamespaceSelector = &metav1.LabelSelector{
			// Note: we are limiting the test extension to be used by the namespace where the test is run.
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "kubernetes.io/metadata.name",
					Operator: metav1.LabelSelectorOpIn,
					Values:   namespaces,
				},
			},
		}
	}
	if err := workloadClusterProxy.GetClient().Create(ctx, cfg); err != nil {
		return fmt.Errorf("failed to create extension config %s", cfg.Name)
	}
	return nil
}

func deployClusterClass(ctx context.Context, specName string, workloadClusterProxy framework.ClusterProxy, e2eConfig *clusterctl.E2EConfig, clusterctlConfigPath string, cluster *clusterv1.Cluster) error {
	variables := map[string]string{
		// This is used to template the name of the ExtensionConfig into the ClusterClass.
		"EXTENSION_CONFIG_NAME": specName,
	}
	if cluster.Spec.Topology.ClassRef.Namespace != cluster.Namespace {
		variables["CLUSTER_CLASS_NAMESPACE"] = cluster.Spec.Topology.ClassRef.Namespace
	}

	clusterctlConfigCopyPath := filepath.Join(filepath.Dir(clusterctlConfigPath), fmt.Sprintf("clusterctl-config-%s.yaml", cluster.Name))
	err := clusterctl.CopyAndAmendClusterctlConfig(ctx, clusterctl.CopyAndAmendClusterctlConfigInput{
		ClusterctlConfigPath: clusterctlConfigPath,
		OutputPath:           clusterctlConfigCopyPath,
		Variables:            variables,
	})
	if err != nil {
		return err
	}

	configClient, err := config.New(ctx, clusterctlConfigCopyPath)
	if err != nil {
		return err
	}

	infraProviderNames := e2eConfig.InfrastructureProviders()
	if len(infraProviderNames) != 1 {
		return fmt.Errorf("expected exactly one infrastructure provider, found %d", len(infraProviderNames))
	}
	infraProviderName := infraProviderNames[0]

	infraProviderVersions := e2eConfig.GetProviderVersions(infraProviderName)
	if len(infraProviderVersions) < 1 {
		return fmt.Errorf("expected at least one infrastructure provider version found %d", len(infraProviderVersions))
	}
	infraProviderVersion := infraProviderVersions[len(infraProviderVersions)-1]

	infraProvider, err := configClient.Providers().Get(infraProviderName, clusterctlv1.InfrastructureProviderType)
	if err != nil {
		return err
	}

	infraRepository, err := repository.New(ctx, infraProvider, configClient)
	if err != nil {
		return err
	}

	file, err := infraRepository.ClusterClasses(infraProviderVersion).Get(ctx, cluster.Spec.Topology.ClassRef.Name, cluster.Spec.Topology.ClassRef.Namespace, false)
	if err != nil {
		return err
	}

	yaml, err := file.Yaml()
	if err != nil {
		return err
	}

	if err := workloadClusterProxy.Create(ctx, yaml); err != nil {
		return fmt.Errorf("failed to deploy clusterclass manifest: %w", err)
	}
	return nil
}
