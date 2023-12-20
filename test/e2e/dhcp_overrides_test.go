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

package e2e

import (
	"encoding/base64"
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

type GuestInfoMetadata struct {
	Network Network `yaml:"network"`
}

type Network struct {
	Ethernets map[string]Ethernet `yaml:"ethernets"`
}

type Ethernet struct {
	DHCP4Overrides DHCPOverrides `yaml:"dhcp4-overrides"`
}

type DHCPOverrides struct {
	SendHostname *bool `yaml:"send-hostname"`
}

var _ = Describe("DHCPOverrides configuration test", func() {
	When("Creating a cluster with DHCPOverrides configured", func() {
		const specName = "dhcp-overrides"
		var namespace *corev1.Namespace

		BeforeEach(func() {
			namespace = setupSpecNamespace(specName)
		})

		AfterEach(func() {
			cleanupSpecNamespace(namespace)
		})

		It("Only configures the network with the provided nameservers", func() {
			clusterName := fmt.Sprintf("%s-%s", specName, util.RandomString(6))
			clusterResources := new(clusterctl.ApplyClusterTemplateAndWaitResult)

			By("Creating a workload cluster")
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "dhcp-overrides",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: ptr.To(int64(1)),
					WorkerMachineCount:       ptr.To(int64(1)),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, clusterResources)

			list := getVSphereVMsForCluster(clusterName, namespace.Name)
			for _, vm := range list.Items {
				metadata, err := getVMMetadata(vm)
				Expect(err).NotTo(HaveOccurred())
				guestInfoMetadata := &GuestInfoMetadata{}
				err = yaml.Unmarshal(metadata, guestInfoMetadata)
				Expect(err).NotTo(HaveOccurred())
				Expect(guestInfoMetadata.Network.Ethernets["id0"].DHCP4Overrides.SendHostname).NotTo(BeNil())
				Expect(*guestInfoMetadata.Network.Ethernets["id0"].DHCP4Overrides.SendHostname).To(BeFalse())
			}
		})
	})
})

func getVMMetadata(vm infrav1.VSphereVM) ([]byte, error) {
	vmObj, err := vsphereFinder.VirtualMachine(ctx, vm.Name)
	Expect(err).NotTo(HaveOccurred())

	properties := []string{}
	vmInfo := mo.VirtualMachine{}
	pc := property.DefaultCollector(vsphereClient.Client)
	err = pc.RetrieveOne(ctx, vmObj.Reference(), properties, &vmInfo)
	Expect(err).NotTo(HaveOccurred())

	for _, extraConfig := range vmInfo.Config.ExtraConfig {
		configValue := extraConfig.GetOptionValue()
		if configValue.Key == "guestinfo.metadata" {
			metadata, err := base64.StdEncoding.DecodeString(configValue.Value.(string))
			Expect(err).NotTo(HaveOccurred())
			return metadata, nil
		}
	}

	return []byte{}, fmt.Errorf("Expected ExtraConfig for %s to have guestinfo.metadata, but it was not found", vmInfo.Name)
}
