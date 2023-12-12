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

package vmoperator

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

var ctx = ctrl.SetupSignalHandler()

func TestCAPVServices(t *testing.T) {
	RegisterFailHandler(Fail)

	// Setting logger so we can set the log level to 5 to cover the code paths
	// which are only executed when log level is set to >= 5.
	ctrl.SetLogger(klog.Background())
	_, err := logs.GlogSetter("5")
	Expect(err).ToNot(HaveOccurred())

	reporterConfig := types.NewDefaultReporterConfig()
	if artifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		reporterConfig.JUnitReport = filepath.Join(artifactFolder, "junit.ginkgo.pkg_services_vmoperator.xml")
	}
	RunSpecs(t, "VMOperator Services Suite", reporterConfig)
}
