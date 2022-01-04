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

//nolint:unused
package builder

import (
	goctx "context"
	"encoding/json"
	"os/exec"
	"path"
	"path/filepath"
	goruntime "runtime"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	vmwarecontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/vmware"
)

// TestSuite is used for unit and integration testing builder.
type TestSuite struct {
	goctx.Context
	integrationTestClient client.Client
	envTest               envtest.Environment
	config                *rest.Config
	flags                 TestFlags
	newReconcilerFn       NewReconcilerFunc
	webhookName           string
}

func (s *TestSuite) isWebhookTest() bool {
	return s.webhookName != ""
}

func (s *TestSuite) GetEnvTestConfg() *rest.Config {
	return s.config
}

type Reconciler interface {
	ReconcileNormal(ctx *vmwarecontext.GuestClusterContext) (reconcile.Result, error)
}

// NewReconcilerFunc is a base type for functions that return a reconciler.
type NewReconcilerFunc func() Reconciler

// NewTestSuiteForController returns a new test suite used for unit and
// integration testing controllers created using the "pkg/builder"
// package.
func NewTestSuiteForController(newReconcilerFn NewReconcilerFunc) *TestSuite {
	testSuite := &TestSuite{
		Context: goctx.Background(),
	}
	testSuite.init(newReconcilerFn)

	if testSuite.flags.UnitTestsEnabled {
		if newReconcilerFn == nil {
			panic("newReconcilerFn is nil")
		}
	}

	return testSuite
}

func (s *TestSuite) SetIntegrationTestClient(integrationTestClient client.Client) {
	s.integrationTestClient = integrationTestClient
}

var (
	scheme *runtime.Scheme
)

func (s *TestSuite) init(newReconcilerFn NewReconcilerFunc) {
	s.flags = GetTestFlags()
	s.newReconcilerFn = newReconcilerFn

	_, filename, _, _ := goruntime.Caller(0) //nolint
	root := path.Join(path.Dir(filename), "..", "..", "..")
	clusterAPIDir := findModuleDir("sigs.k8s.io/cluster-api")

	s.envTest = envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(root, "config", "supervisor", "crd"),
			filepath.Join(root, "config", "deployments", "integration-tests", "crds"),
			filepath.Join(clusterAPIDir, "config", "crd", "bases"),
		},
		Scheme: scheme,
	}
}

func findModuleDir(module string) string {
	cmd := exec.Command("go", "mod", "download", "-json", module)
	out, err := cmd.Output()
	if err != nil {
		klog.Fatalf("Failed to run go mod to find module %q directory", module)
	}
	info := struct{ Dir string }{}
	if err := json.Unmarshal(out, &info); err != nil {
		klog.Fatalf("Failed to unmarshal output from go mod command: %v", err)
	} else if info.Dir == "" {
		klog.Fatalf("Failed to find go module %q directory, received %v", module, string(out))
	}
	return info.Dir
}

// NewUnitTestContextForController returns a new unit test context for this
// suite's reconciler.
//
// Returns nil if unit testing is disabled.
func (s *TestSuite) NewUnitTestContextForController(initObjects ...client.Object) *UnitTestContextForController {
	return s.NewUnitTestContextForControllerWithVSphereCluster(nil, false, initObjects...)
}

// NewUnitTestContextForControllerWithPrototypeCluster returns a new unit test
// context with a prototype cluster for this suite's reconciler. This prototype cluster
// helps controllers that do not wish to invoke the full TanzuKubernetesCluster
// spec reconciliation.
//
// Returns nil if unit testing is disabled.
func (s *TestSuite) NewUnitTestContextForControllerWithPrototypeCluster(initObjects ...client.Object) *UnitTestContextForController {
	return s.NewUnitTestContextForControllerWithVSphereCluster(nil, true, initObjects...)
}

// NewUnitTestContextForControllerWithVSphereCluster returns a new unit test context for this
// suite's reconciler initialized with the given vspherecluster.
//
// Returns nil if unit testing is disabled.
func (s *TestSuite) NewUnitTestContextForControllerWithVSphereCluster(vsphereCluster *vmwarev1.VSphereCluster, prototypeCluster bool, initObjects ...client.Object) *UnitTestContextForController {
	if s.flags.UnitTestsEnabled {
		ctx := NewUnitTestContextForController(s.newReconcilerFn, vsphereCluster, prototypeCluster, initObjects, nil)
		reconcileNormalAndExpectSuccess(ctx)
		// Update the VSphereCluster and its status in the fake client.
		Expect(ctx.Client.Update(ctx, ctx.VSphereCluster)).To(Succeed())
		Expect(ctx.Client.Status().Update(ctx, ctx.VSphereCluster)).To(Succeed())

		return ctx
	}
	return nil
}

func reconcileNormalAndExpectSuccess(ctx *UnitTestContextForController) {
	// Manually invoke the reconciliation. This is poor design, but in order
	// to support unit testing with a minimum set of dependencies that does
	// not include the Kubernetes envtest package, this is required.
	Expect(ctx.ReconcileNormal()).ShouldNot(HaveOccurred())
}
