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

package extra

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	ginkgotypes "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vim25/types"
)

func TestExtra(t *testing.T) {
	RegisterFailHandler(Fail)

	reporterConfig := ginkgotypes.NewDefaultReporterConfig()
	if artifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		reporterConfig.JUnitReport = filepath.Join(artifactFolder, "junit.ginkgo.pkg_services_govmomi_extra.xml")
	}
	RunSpecs(t, "Extra Suite", reporterConfig)
}

type ConfigInitFn func(*Config, string)

var _ = Describe("Config_SetCustomVMXKeys", func() {
	Context("we try to set custom keys in the config", func() {
		var config Config
		customConfigKeys := map[string]string{
			"customKey1": "customVal1",
			"customKey2": "customKey2",
		}

		It("adds the keys to the config", func() {
			err := config.SetCustomVMXKeys(customConfigKeys)

			Expect(err).ToNot(HaveOccurred())

			for k, v := range customConfigKeys {
				Expect(config).To(ContainElement(&types.OptionValue{
					Key:   k,
					Value: v,
				}))
			}
		})
	})
})

var _ = Describe("Config_SetCloudInitUserData", func() {
	ConfigInitFnTester(
		func(config *Config, s string) {
			config.SetCloudInitUserData([]byte(s))
		},
		"SetCloudInitUserData",
		"guestinfo.userdata",
		"guestinfo.userdata.encoding",
	)
})

var _ = Describe("Config_SetCloudInitMetadata", func() {
	ConfigInitFnTester(func(config *Config, s string) {
		config.SetCloudInitMetadata([]byte(s))
	},
		"SetCloudInitMetadata",
		"guestinfo.metadata",
		"guestinfo.metadata.encoding",
	)
})

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// ConfigInitFnTester is a common testing method for config.SetCloudInitUserData and config.SetCloudInitMetadata.
func ConfigInitFnTester(method ConfigInitFn, methodName string, dataKey string, encodingKey string) {
	const sampleData = "some sample data, "
	var expectedData = base64Encode(sampleData)

	Context(fmt.Sprintf("we call %q with some non-encoded sample data", methodName), func() {
		var config Config
		method(&config, sampleData)

		It("must set 2 keys in the config", func() {
			Expect(config).To(HaveLen(2))
		})

		It(fmt.Sprintf("must set data as a base64 encoded string with the key %q", dataKey), func() {
			Expect(config).To(ContainElement(&types.OptionValue{
				Key:   dataKey,
				Value: expectedData,
			}))
		})

		It("must set a key to indicate base64 encoding of the data", func() {
			Expect(config).To(ContainElement(&types.OptionValue{
				Key:   encodingKey,
				Value: "base64",
			}))
		})
	})

	Context(fmt.Sprintf("We call %q with some pre-encoded data (single pass)", methodName), func() {
		var config Config
		preEncodedData := base64Encode(sampleData)
		method(&config, preEncodedData)

		It("does not encode the data further on storing", func() {
			Expect(config).To(ContainElement(&types.OptionValue{
				Key:   dataKey,
				Value: preEncodedData,
			}))
		})
	})

	Context(fmt.Sprintf("We call %q with some pre-encoded data (multiple passes)", methodName), func() {
		var config Config
		const passCount = 5
		multiPassEncodedData := sampleData
		for i := 0; i < passCount; i++ {
			multiPassEncodedData = base64Encode(multiPassEncodedData)
		}

		method(&config, multiPassEncodedData)

		It("saves data with a single pass of encoding", func() {
			Expect(config).To(ContainElement(&types.OptionValue{
				Key:   dataKey,
				Value: expectedData,
			}))
		})
	})
}
