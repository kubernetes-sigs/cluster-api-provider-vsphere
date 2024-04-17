/*
Copyright 2024 The Kubernetes Authors.

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
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	vsphereip "sigs.k8s.io/cluster-api-provider-vsphere/test/framework/ip"
	vspherevcsim "sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vcsim"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vmoperator"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type setupOptions struct {
	additionalIPVariableNames []string
	gatewayIPVariableName     string
}

// SetupOption is a configuration option supplied to Setup.
type SetupOption func(*setupOptions)

// WithIP instructs Setup to allocate another IP and store it into the provided variableName
// NOTE: Setup always allocate an IP for CONTROL_PLANE_ENDPOINT_IP.
func WithIP(variableName string) SetupOption {
	return func(o *setupOptions) {
		o.additionalIPVariableNames = append(o.additionalIPVariableNames, variableName)
	}
}

// WithGateway instructs Setup to store the Gateway IP from IPAM into the provided variableName.
func WithGateway(variableName string) SetupOption {
	return func(o *setupOptions) {
		o.gatewayIPVariableName = variableName
	}
}

type testSettings struct {
	ClusterctlConfigPath     string
	PostNamespaceCreatedFunc func(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string)
	FlavorForMode            func(flavor string) string
}

// Setup for the specific test.
func Setup(specName string, f func(testSpecificSettings func() testSettings), opts ...SetupOption) {
	options := &setupOptions{}
	for _, o := range opts {
		o(options)
	}

	var (
		testSpecificClusterctlConfigPath string
		testSpecificIPAddressClaims      vsphereip.AddressClaims
		testSpecificVariables            map[string]string
		postNamespaceCreatedFunc         func(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string)
	)
	BeforeEach(func() {
		Byf("Setting up test env for %s", specName)
		switch testTarget {
		case VCenterTestTarget:
			Byf("Getting IP for %s", strings.Join(append([]string{vsphereip.ControlPlaneEndpointIPVariable}, options.additionalIPVariableNames...), ","))
			// get IPs from the in cluster address manager
			testSpecificIPAddressClaims, testSpecificVariables = inClusterAddressManager.ClaimIPs(ctx, vsphereip.WithGateway(options.gatewayIPVariableName), vsphereip.WithIP(options.additionalIPVariableNames...))
		case VCSimTestTarget:
			c := bootstrapClusterProxy.GetClient()

			// get IPs from the vcsim controller
			// NOTE: ControlPlaneEndpointIP is the first claim in the returned list (this assumption is used below).
			Byf("Getting IP for %s", strings.Join(append([]string{vsphereip.ControlPlaneEndpointIPVariable}, options.additionalIPVariableNames...), ","))
			testSpecificIPAddressClaims, testSpecificVariables = vcsimAddressManager.ClaimIPs(ctx, vsphereip.WithIP(options.additionalIPVariableNames...))

			// variables derived from the vCenterSimulator
			vCenterSimulator, err := vspherevcsim.Get(ctx, c)
			Expect(err).ToNot(HaveOccurred(), "Failed to get VCenterSimulator")

			Byf("Creating EnvVar %s", klog.KRef(metav1.NamespaceDefault, specName))
			envVar := &vcsimv1.EnvVar{
				ObjectMeta: metav1.ObjectMeta{
					Name:      specName,
					Namespace: metav1.NamespaceDefault,
				},
				Spec: vcsimv1.EnvVarSpec{
					VCenterSimulator: &vcsimv1.NamespacedRef{
						Namespace: vCenterSimulator.Namespace,
						Name:      vCenterSimulator.Name,
					},
					ControlPlaneEndpoint: vcsimv1.NamespacedRef{
						Namespace: testSpecificIPAddressClaims[0].Namespace,
						Name:      testSpecificIPAddressClaims[0].Name,
					},
					// NOTE: we are omitting VMOperatorDependencies because it is not created yet (it will be created by the PostNamespaceCreated hook)
					// But this is not a issue because a default dependenciesConfig that works for vcsim will be automatically used.
				},
			}

			err = c.Create(ctx, envVar)
			Expect(err).ToNot(HaveOccurred(), "Failed to create EnvVar")

			Eventually(func() bool {
				if err := c.Get(ctx, crclient.ObjectKeyFromObject(envVar), envVar); err != nil {
					return false
				}
				return len(envVar.Status.Variables) > 0
			}, 30*time.Second, 5*time.Second).Should(BeTrue(), "Failed to get EnvVar %s", klog.KObj(envVar))

			Byf("Setting test variables for %s", specName)
			for k, v := range envVar.Status.Variables {
				// ignore variables that will be set later on by the test
				if sets.New("NAMESPACE", "CLUSTER_NAME", "KUBERNETES_VERSION", "CONTROL_PLANE_MACHINE_COUNT", "WORKER_MACHINE_COUNT").Has(k) {
					continue
				}

				// unset corresponding env variable (that in CI contains VMC data), so we are sure we use the value for vcsim
				if strings.HasPrefix(k, "VSPHERE_") {
					Expect(os.Unsetenv(k)).To(Succeed())
				}

				testSpecificVariables[k] = v
			}
		}

		if testMode == SupervisorTestMode {
			postNamespaceCreatedFunc = setupNamespaceWithVMOperatorDependenciesVCenter
			if testTarget == VCSimTestTarget {
				postNamespaceCreatedFunc = setupNamespaceWithVMOperatorDependenciesVCSim
			}

			if testSpecificVariables == nil {
				testSpecificVariables = map[string]string{}
			}

			// Update the CLUSTER_CLASS_NAME variable adding the supervisor suffix.
			if e2eConfig.HasVariable("CLUSTER_CLASS_NAME") {
				testSpecificVariables["CLUSTER_CLASS_NAME"] = fmt.Sprintf("%s-supervisor", e2eConfig.GetVariable("CLUSTER_CLASS_NAME"))
			}
		}

		// Create a new clusterctl config file based on the passed file and add the new variables for the IPs.
		testSpecificClusterctlConfigPath = fmt.Sprintf("%s-%s.yaml", strings.TrimSuffix(clusterctlConfigPath, ".yaml"), specName)
		Byf("Writing a new clusterctl config to %s", testSpecificClusterctlConfigPath)
		copyAndAmendClusterctlConfig(ctx, copyAndAmendClusterctlConfigInput{
			ClusterctlConfigPath: clusterctlConfigPath,
			OutputPath:           testSpecificClusterctlConfigPath,
			Variables:            testSpecificVariables,
		})
	})
	defer AfterEach(func() {
		Byf("Cleaning up test env for %s", specName)
		switch testTarget {
		case VCenterTestTarget:
			// cleanup IPs/controlPlaneEndpoint created by the in cluster ipam provider.
			Expect(inClusterAddressManager.Cleanup(ctx, testSpecificIPAddressClaims)).To(Succeed())
		case VCSimTestTarget:
			// cleanup IPs/controlPlaneEndpoint created by the vcsim controller manager.
			Expect(vcsimAddressManager.Cleanup(ctx, testSpecificIPAddressClaims)).To(Succeed())
		}
	})

	// NOTE: it is required to use a function to pass the testSpecificClusterctlConfigPath value into the test func,
	// so when the test is executed the func could get the value set into the BeforeEach block above.
	// If instead we pass the value directly, the test func will get the value at the moment of the initial parsing of
	// the Ginkgo node tree, which is an empty string (the BeforeEach block above are not run during initial parsing).
	f(func() testSettings {
		return testSettings{
			ClusterctlConfigPath:     testSpecificClusterctlConfigPath,
			PostNamespaceCreatedFunc: postNamespaceCreatedFunc,
			FlavorForMode: func(flavor string) string {
				if testMode == SupervisorTestMode {
					// This assumes all the supervisor flavors have the name of the corresponding govmomi flavor + "-supervisor" suffix
					if flavor == "" {
						return "supervisor"
					}
					return fmt.Sprintf("%s-supervisor", flavor)
				}
				return flavor
			},
		}
	})
}

func setupNamespaceWithVMOperatorDependenciesVCSim(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string) {
	c := managementClusterProxy.GetClient()

	vCenterSimulator, err := vspherevcsim.Get(ctx, bootstrapClusterProxy.GetClient())
	Expect(err).ToNot(HaveOccurred(), "Failed to get VCenterSimulator")

	Byf("Creating VMOperatorDependencies %s", klog.KRef(workloadClusterNamespace, "vcsim"))
	dependenciesConfig := &vcsimv1.VMOperatorDependencies{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vcsim",
			Namespace: workloadClusterNamespace,
		},
		Spec: vcsimv1.VMOperatorDependenciesSpec{
			VCenterSimulatorRef: &vcsimv1.NamespacedRef{
				Namespace: vCenterSimulator.Namespace,
				Name:      vCenterSimulator.Name,
			},
		},
	}
	err = c.Create(ctx, dependenciesConfig)
	Expect(err).ToNot(HaveOccurred(), "Failed to create VMOperatorDependencies")

	Eventually(func() bool {
		if err := c.Get(ctx, crclient.ObjectKeyFromObject(dependenciesConfig), dependenciesConfig); err != nil {
			return false
		}
		return dependenciesConfig.Status.Ready
	}, 30*time.Second, 5*time.Second).Should(BeTrue(), "Failed to get VMOperatorDependencies on namespace %s", workloadClusterNamespace)
}

func setupNamespaceWithVMOperatorDependenciesVCenter(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string) {
	c := managementClusterProxy.GetClient()

	Byf("Creating VMOperatorDependencies %s", klog.KRef(workloadClusterNamespace, "vcsim"))
	mustParseInt64 := func(s string) int64 {
		i, err := strconv.Atoi(s)
		if err != nil {
			panic(fmt.Sprintf("%q must be a valid int64", s))
		}
		return int64(i)
	}

	dependenciesConfig := &vcsimv1.VMOperatorDependencies{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vcenter",
			Namespace: workloadClusterNamespace,
		},
		Spec: vcsimv1.VMOperatorDependenciesSpec{
			VCenter: &vcsimv1.VCenterSpec{
				// NOTE: variables from E2E.sh + presets (or variables overrides when running tests locally)
				ServerURL:  net.JoinHostPort(e2eConfig.GetVariable("VSPHERE_SERVER"), "443"),
				Username:   e2eConfig.GetVariable("VSPHERE_USERNAME"),
				Password:   e2eConfig.GetVariable("VSPHERE_PASSWORD"),
				Thumbprint: e2eConfig.GetVariable("VSPHERE_TLS_THUMBPRINT"),
				// NOTE: variables from e2e config (or variables overrides when running tests locally)
				Datacenter:   e2eConfig.GetVariable("VSPHERE_DATACENTER"),
				Cluster:      e2eConfig.GetVariable("VSPHERE_COMPUTE_CLUSTER"),
				Folder:       e2eConfig.GetVariable("VSPHERE_FOLDER"),
				ResourcePool: e2eConfig.GetVariable("VSPHERE_RESOURCE_POOL"),
				ContentLibrary: vcsimv1.ContentLibraryConfig{
					Name:      e2eConfig.GetVariable("VSPHERE_CONTENT_LIBRARY"),
					Datastore: e2eConfig.GetVariable("VSPHERE_DATASTORE"),
					// NOTE: when running on vCenter the vm-operator automatically creates VirtualMachine objects for the content library.
					Items: []vcsimv1.ContentLibraryItemConfig{},
				},
				DistributedPortGroupName: e2eConfig.GetVariable("VSPHERE_DISTRIBUTED_PORT_GROUP"),
			},
			StorageClasses: []vcsimv1.StorageClass{
				{
					Name:          e2eConfig.GetVariable("VSPHERE_STORAGE_CLASS"),
					StoragePolicy: e2eConfig.GetVariable("VSPHERE_STORAGE_POLICY"),
				},
			},
			VirtualMachineClasses: []vcsimv1.VirtualMachineClass{
				{
					Name:   e2eConfig.GetVariable("VSPHERE_MACHINE_CLASS_NAME"),
					Cpus:   mustParseInt64(e2eConfig.GetVariable("VSPHERE_MACHINE_CLASS_CPU")),
					Memory: resource.MustParse(e2eConfig.GetVariable("VSPHERE_MACHINE_CLASS_MEMORY")),
				},
				{
					Name:   e2eConfig.GetVariable("VSPHERE_MACHINE_CLASS_NAME_CONFORMANCE"),
					Cpus:   mustParseInt64(e2eConfig.GetVariable("VSPHERE_MACHINE_CLASS_CPU_CONFORMANCE")),
					Memory: resource.MustParse(e2eConfig.GetVariable("VSPHERE_MACHINE_CLASS_MEMORY_CONFORMANCE")),
				},
			},
		},
	}

	items := e2eConfig.GetVariable("VSPHERE_CONTENT_LIBRARY_ITEMS")
	if items != "" {
		for _, i := range strings.Split(e2eConfig.GetVariable("VSPHERE_CONTENT_LIBRARY_ITEMS"), ",") {
			dependenciesConfig.Spec.VCenter.ContentLibrary.Items = append(dependenciesConfig.Spec.VCenter.ContentLibrary.Items, vcsimv1.ContentLibraryItemConfig{
				Name:     i,
				ItemType: "ovf",
			})
		}
	}

	err := vmoperator.ReconcileDependencies(ctx, c, dependenciesConfig)
	Expect(err).ToNot(HaveOccurred(), "Failed to reconcile VMOperatorDependencies")
}

// Note: Copy-paste from CAPI below.

// copyAndAmendClusterctlConfigInput is the input for copyAndAmendClusterctlConfig.
type copyAndAmendClusterctlConfigInput struct {
	ClusterctlConfigPath string
	OutputPath           string
	Variables            map[string]string
}

// copyAndAmendClusterctlConfig copies the clusterctl-config from ClusterctlConfigPath to
// OutputPath and adds the given Variables.
func copyAndAmendClusterctlConfig(_ context.Context, input copyAndAmendClusterctlConfigInput) {
	// Read clusterctl config from ClusterctlConfigPath.
	clusterctlConfigFile := &clusterctlConfig{
		Path: input.ClusterctlConfigPath,
	}
	clusterctlConfigFile.read()

	// Overwrite variables.
	if clusterctlConfigFile.Values == nil {
		clusterctlConfigFile.Values = map[string]interface{}{}
	}
	for key, value := range input.Variables {
		clusterctlConfigFile.Values[key] = value
	}

	// Write clusterctl config to OutputPath.
	clusterctlConfigFile.Path = input.OutputPath
	clusterctlConfigFile.write()
}

type clusterctlConfig struct {
	Path   string
	Values map[string]interface{}
}

// write writes a clusterctl config file to disk.
func (c *clusterctlConfig) write() {
	data, err := yaml.Marshal(c.Values)
	Expect(err).ToNot(HaveOccurred(), "Failed to marshal the clusterctl config file")

	Expect(os.WriteFile(c.Path, data, 0600)).To(Succeed(), "Failed to write the clusterctl config file")
}

// read reads a clusterctl config file from disk.
func (c *clusterctlConfig) read() {
	data, err := os.ReadFile(c.Path)
	Expect(err).ToNot(HaveOccurred())

	err = yaml.Unmarshal(data, &c.Values)
	Expect(err).ToNot(HaveOccurred(), "Failed to unmarshal the clusterctl config file")
}
