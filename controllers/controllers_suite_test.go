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
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers"
	// +kubebuilder:scaffold:imports
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var (
	testEnv *helpers.TestEnvironment
)

func TestMain(m *testing.M) {
	setup()
	defer func() {
		fmt.Println("Tearing down test suite")
		teardown()
	}()
	code := m.Run()
	os.Exit(code)
}

func setup() {
	fmt.Println("Creating new test environment")
	utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))

	testEnv = helpers.NewTestEnvironment()

	if err := AddClusterControllerToManager(testEnv.GetContext(), testEnv.Manager); err != nil {
		panic(fmt.Sprintf("unable to setup VsphereCluster controller: %v", err))
	}
	if err := AddMachineControllerToManager(testEnv.GetContext(), testEnv.Manager); err != nil {
		panic(fmt.Sprintf("unable to setup VsphereMachine controller: %v", err))
	}
	if err := AddVMControllerToManager(testEnv.GetContext(), testEnv.Manager); err != nil {
		panic(fmt.Sprintf("unable to setup VsphereVM controller: %v", err))
	}
	if err := AddHAProxyLoadBalancerControllerToManager(testEnv.GetContext(), testEnv.Manager); err != nil {
		panic(fmt.Sprintf("unable to setup HAProxyLB controller: %v", err))
	}

	go func() {
		fmt.Println("Starting the manager")
		if err := testEnv.StartManager(); err != nil {
			panic(fmt.Sprintf("failed to start the envtest manager: %v", err))
		}
	}()

	// wait for webhook port to be open prior to running tests
	testEnv.WaitForWebhooks()
}

func teardown() {
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop envtest: %v", err))
	}
}
