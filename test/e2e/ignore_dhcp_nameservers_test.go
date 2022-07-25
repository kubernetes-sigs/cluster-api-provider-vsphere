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
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/find"
	"golang.org/x/crypto/ssh"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"

	types "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

var _ = Describe("IgnoreDHCPNameservers configuration test", func() {
	When("Creating a cluster with IgnoreDHCPNameservers configured", func() {
		It("Only configures the network with the provided nameservers", func() {
			specName := "ignore-dhcp-nameservers"
			namespace := setupSpecNamespace(specName)
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
					Flavor:                   "ignore-dhcp-nameservers",
					Namespace:                namespace.Name,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(1),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			}, clusterResources)

			list := getVSphereVMsForCluster(clusterName, namespace.Name)
			for _, vm := range list.Items {
				vmIP := getVMIPAddr(vm)
				dnsServers := getVMNameservers(vmIP)

				Expect(dnsServers).To(ConsistOf([]string{"8.8.8.8", "8.8.4.4"}))
			}
		})
	})
})

func getVMIPAddr(vm types.VSphereVM) string {
	finder := find.NewFinder(vsphereClient.Client, false)
	dc, err := finder.Datacenter(ctx, vm.Spec.Datacenter)
	Expect(err).NotTo(HaveOccurred())
	finder.SetDatacenter(dc)

	vmObj, err := finder.VirtualMachine(ctx, fmt.Sprintf("/%s/vm/%s/%s", vm.Spec.Datacenter, vm.Spec.Folder, vm.Name))
	Expect(err).NotTo(HaveOccurred())
	ip, err := vmObj.WaitForIP(ctx, true)
	Expect(err).NotTo(HaveOccurred())

	return ip
}

func getVMNameservers(vmIP string) []string {
	privateKeyPath := e2eConfig.GetVariable("VSPHERE_SSH_PRIVATE_KEY")
	privateKey, err := os.ReadFile(privateKeyPath)
	Expect(err).NotTo(HaveOccurred())

	key, err := ssh.ParsePrivateKey(privateKey)
	Expect(err).NotTo(HaveOccurred())

	sshConfig := &ssh.ClientConfig{
		User:            "capv",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(vmIP, "22"), sshConfig)
	Expect(err).NotTo(HaveOccurred())

	session, err := client.NewSession()
	Expect(err).NotTo(HaveOccurred())
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b

	// get just the nameserver addresses
	err = session.Run("sudo cat /run/systemd/resolve/resolv.conf | grep nameserver | cut -d' ' -f2")
	Expect(err).NotTo(HaveOccurred())

	return strings.Split(b.String(), " ")
}
