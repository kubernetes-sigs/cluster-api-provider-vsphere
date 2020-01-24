/*
Copyright 2019 The Kubernetes Authors.

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
	"flag"
	"io/ioutil"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	kindx "sigs.k8s.io/cluster-api-provider-vsphere/test/e2e/kind"
)

var (
	mgmt       framework.ManagementCluster
	mgmtClient client.Client
	configPath string
	config     *framework.Config
	ctx        = context.Background()
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "e2e.conf", "path to the e2e config file")
}

func TestCAPV(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CAPV e2e Suite")
}

var _ = BeforeSuite(func() {
	By("loading e2e config")
	data, err := ioutil.ReadFile(configPath)
	Expect(err).ShouldNot(HaveOccurred())
	config, err = framework.LoadConfig(data)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(config).ShouldNot(BeNil())

	By("applying e2e config defaults")
	config.Defaults()

	By("cleaning up previous kind cluster")
	kindx.TeardownIfExists(ctx, config.ManagementClusterName)

	By("initializing the vSphere session", initVSphereSession)

	By("initializing the runtime.Scheme")
	scheme := runtime.NewScheme()
	Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	mgmt = framework.InitManagementCluster(ctx, &framework.InitManagementClusterInput{
		ComponentGenerators: []framework.ComponentGenerator{credentialsGenerator{}},
		Config:              *config,
		Scheme:              scheme,
	})

	mgmtClient, err = mgmt.GetClient()
	Expect(err).ShouldNot(HaveOccurred())
	Expect(mgmtClient).ShouldNot(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the management cluster")
	mgmt.Teardown(ctx)
})
