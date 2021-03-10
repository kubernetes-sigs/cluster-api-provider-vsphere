/*
Copyright 2020 The Kubernetes Authors.

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/pbm"
	"github.com/vmware/govmomi/pbm/types"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	vim "github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Cluster Creation using Cluster API quick-start test", func() {

	Byf("Creating single-node control plane with one worker node")
	capi_e2e.QuickStartSpec(context.TODO(), func() capi_e2e.QuickStartSpecInput {
		return capi_e2e.QuickStartSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
		}
	})
})

var _ = Describe("Cluster creation with vSphere validations", func() {
	var namespace *corev1.Namespace

	BeforeEach(func() {
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy can't be nil")
		namespace = setupSpecNamespace("capv-e2e")
	})

	AfterEach(func() {
		cleanupSpecNamespace(namespace)
	})

	It("should create a cluster with storage policy", func() {
		clusterName := fmt.Sprintf("cluster-%s", util.RandomString(6))
		Expect(namespace).NotTo(BeNil())

		ds := getDatastore()
		if ds == nil {
			Fail("unable to retrieve datastore")
			return
		}

		restClient := rest.NewClient(vsphereClient.Client)
		Expect(restClient.Login(ctx, userInfo)).NotTo(HaveOccurred())
		tagsManager := tags.NewManager(restClient)

		cat := tags.Category{Name: fmt.Sprintf("%s-cat", clusterName), Cardinality: "SINGLE"}
		Byf("creating category: %s", cat.Name)
		catID, err := tagsManager.CreateCategory(ctx, &cat)
		Expect(err).NotTo(HaveOccurred())
		cat.ID = catID

		tag := tags.Tag{Name: fmt.Sprintf("%s-tag", clusterName), CategoryID: catID}
		Byf("creating tag: %s", tag.Name)
		tagID, err := tagsManager.CreateTag(ctx, &tag)
		Expect(err).NotTo(HaveOccurred())
		tag.ID = tagID

		By("attaching tag to datastore")
		Expect(tagsManager.AttachTag(ctx, tag.ID, ds.Reference())).NotTo(HaveOccurred())

		// create a storage policy with tag and category
		spName := fmt.Sprintf("%s-sp", clusterName)
		spID := createStoragePolicy(spName, cat.Name, tag.Name)
		if spID == nil {
			Fail("unable to create storage policy")
			return
		}

		// creating machine template first as ApplyClusterTemplateAndWait will wait for machines to become ready
		By("creating vsphere machine template")
		vsphereMachineTemplate := makeVsphereMachineTemplate(clusterName, namespace.Name)
		vsphereMachineTemplate.Spec.Template.Spec.StoragePolicyName = spName
		Expect(bootstrapClusterProxy.GetClient().Create(ctx, vsphereMachineTemplate)).ShouldNot(HaveOccurred())

		By("creating a workload cluster")
		configCluster := defaultConfigCluster(clusterName, namespace.Name)
		configCluster.Flavor = StoragePolicyFlavor

		_ = clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy:                 bootstrapClusterProxy,
			ConfigCluster:                configCluster,
			WaitForClusterIntervals:      e2eConfig.GetIntervals("", "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals("", "wait-control-plane"),
			WaitForMachineDeployments:    e2eConfig.GetIntervals("", "wait-worker-nodes"),
		})

		By("verifying storage policy is used by VMs")
		pbmClient, err := pbm.NewClient(ctx, vsphereClient.Client)
		Expect(err).NotTo(HaveOccurred())
		var res []types.PbmServerObjectRef
		if pbmClient != nil {
			res, err = pbmClient.QueryAssociatedEntity(ctx, types.PbmProfileId{UniqueId: spID.UniqueId}, "virtualMachine")
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(len(res)).To(BeNumerically(">", 0))

		// Delete storage policy, tag and category
		if pbmClient != nil {
			_, _ = pbmClient.DeleteProfile(ctx, []types.PbmProfileId{*spID})
		}
		_ = tagsManager.DeleteTag(ctx, &tag)
		_ = tagsManager.DeleteCategory(ctx, &cat)
	})
})

func createStoragePolicy(spName string, cat string, tag string) *types.PbmProfileId {
	pbmClient, err := pbm.NewClient(ctx, vsphereClient.Client)
	Expect(err).NotTo(HaveOccurred())

	spec := types.PbmCapabilityProfileCreateSpec{
		Name: spName,
	}
	Byf("creating storage policy: %s", spec.Name)
	spec.ResourceType.ResourceType = string(types.PbmProfileResourceTypeEnumSTORAGE)
	id := fmt.Sprintf("com.vmware.storage.tag.%s.property", cat)
	instance := types.PbmCapabilityInstance{
		Id: types.PbmCapabilityMetadataUniqueId{
			Namespace: "http://www.vmware.com/storage/tag",
			Id:        cat,
		},
		Constraint: []types.PbmCapabilityConstraintInstance{{
			PropertyInstance: []types.PbmCapabilityPropertyInstance{{
				Id: id,
				Value: types.PbmCapabilityDiscreteSet{
					Values: []vim.AnyType{tag},
				},
			}},
		}},
	}

	spec.Constraints = &types.PbmCapabilitySubProfileConstraints{
		SubProfiles: []types.PbmCapabilitySubProfile{{
			Name:       "Tag based placement",
			Capability: []types.PbmCapabilityInstance{instance},
		}},
	}

	spID, err := pbmClient.CreateProfile(ctx, spec)
	Expect(err).NotTo(HaveOccurred())

	return spID
}

func getDatastore() *object.Datastore {
	// fetch datastore defined for the quickstart test or default
	ds, err := vsphereFinder.DatastoreOrDefault(ctx, getVariable(VsphereDatastore))
	Expect(err).NotTo(HaveOccurred())

	return ds
}

func defaultConfigCluster(clusterName, namespace string) clusterctl.ConfigClusterInput {
	return clusterctl.ConfigClusterInput{
		LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
		ClusterctlConfigPath:     clusterctlConfigPath,
		KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
		InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
		Flavor:                   clusterctl.DefaultFlavor,
		Namespace:                namespace,
		ClusterName:              clusterName,
		KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
		ControlPlaneMachineCount: pointer.Int64Ptr(1),
		WorkerMachineCount:       pointer.Int64Ptr(0),
	}
}

func makeVsphereMachineTemplate(clusterName, namespace string) *v1alpha3.VSphereMachineTemplate {
	machine := &v1alpha3.VSphereMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: v1alpha3.VSphereMachineTemplateSpec{
			Template: v1alpha3.VSphereMachineTemplateResource{
				Spec: v1alpha3.VSphereMachineSpec{
					VirtualMachineCloneSpec: v1alpha3.VirtualMachineCloneSpec{
						CloneMode:  v1alpha3.LinkedClone,
						Datacenter: getVariable(VsphereDatacenter),
						DiskGiB:    25,
						Folder:     getVariable(VsphereFolder),
						MemoryMiB:  8192,
						Network: v1alpha3.NetworkSpec{
							Devices: []v1alpha3.NetworkDeviceSpec{
								{
									DHCP4:       true,
									NetworkName: getVariable(VsphereNetwork),
								},
							},
						},
						NumCPUs:      2,
						ResourcePool: getVariable(VsphereResourcePool),
						Server:       getVariable(VsphereServer),
						Template:     getVariable(VsphereTemplate),
						Thumbprint:   getVariable(VsphereTLSThumbprint),
					},
				},
			},
		},
	}

	return machine
}
