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

package v1alpha3_test

import (
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-vsphere/apis/v1alpha3"
)

var unmarshalWarnAsFatal = []v1alpha3.UnmarshalINIOptionFunc{v1alpha3.WarnAsFatal}

func errDeprecated(section, key string) error {
	return errors.Errorf("warning:\ncan't store data at section \"%s\", variable \"%s\"\n", section, key)
}

type codecTestCase struct {
	testName         string
	iniString        string
	configObj        v1alpha3.CPIConfig
	expectedError    error
	unmarshalOptions []v1alpha3.UnmarshalINIOptionFunc
}

func TestMarshalINI(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	testcases := []codecTestCase{
		{
			testName: "Username and password in global section",
			iniString: `[Global]
user = "user"
password = "password"
datacenters = "us-west"
cluster-id = "cluster-namespace/cluster-name"

[VirtualCenter "0.0.0.0"]

[Workspace]
server = "0.0.0.0"
datacenter = "us-west"
folder = "kubernetes"
default-datastore = "default"

`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
					ClusterID:   "cluster-namespace/cluster-name",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
					Datastore:  "default",
				},
			},
		},
		{
			testName: "Username and password in vCenter section",
			iniString: `[Global]
insecure-flag = true
port = "443"
datacenters = "us-west"

[VirtualCenter "0.0.0.0"]
user = "user"
password = "password"

[Workspace]
server = "0.0.0.0"
datacenter = "us-west"
folder = "kubernetes"

`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Port:        "443",
					Insecure:    true,
					Datacenters: "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Username: "user",
						Password: "password",
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "SecretName and SecretNamespace",
			iniString: `[Global]
secret-name = "vccreds"
secret-namespace = "kube-system"
datacenters = "us-west"

[VirtualCenter "0.0.0.0"]

[Workspace]
server = "0.0.0.0"
datacenter = "us-west"
folder = "kubernetes"

`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "SecretName and SecretNamespace with Username missing",
			iniString: `[Global]
insecure-flag = true
secret-name = "vccreds"
secret-namespace = "kube-system"
port = "443"
datacenters = "us-west"

[VirtualCenter "0.0.0.0"]
password = "password"

[Workspace]
server = "0.0.0.0"
datacenter = "us-west"
folder = "kubernetes"

`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Port:            "443",
					Insecure:        true,
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Password: "password",
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "Multiple virtual centers with different thumbprints",
			iniString: `[Global]
user = "user"
password = "password"
datacenters = "us-west"

[VirtualCenter "0.0.0.0"]
thumbprint = "thumbprint:0"

[VirtualCenter "1.1.1.1"]
thumbprint = "thumbprint:1"

[VirtualCenter "no_thumbprint"]

[Workspace]
server = "0.0.0.0"
datacenter = "us-west"
folder = "kubernetes"

`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Thumbprint: "thumbprint:0",
					},
					"no_thumbprint": {},
					"1.1.1.1": {
						Thumbprint: "thumbprint:1",
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "Multiple vCenters using global CA cert",
			iniString: `[Global]
secret-name = "vccreds"
secret-namespace = "kube-system"
ca-file = "/some/path/to/my/trusted/ca.pem"
datacenters = "us-west"

[VirtualCenter "0.0.0.0"]

[VirtualCenter "1.1.1.1"]

[Workspace]
server = "0.0.0.0"
datacenter = "us-west"
folder = "kubernetes"

`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Datacenters:     "us-west",
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					CAFile:          "/some/path/to/my/trusted/ca.pem",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {},
					"1.1.1.1": {},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
				ProviderConfig: v1alpha3.CPIProviderConfig{
					Cloud: &v1alpha3.CPICloudConfig{
						ControllerImage: "test",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			buf, err := tc.configObj.MarshalINI()
			if err != nil {
				if tc.expectedError == nil {
					g.Expect(err).ShouldNot(
						gomega.HaveOccurred(),
						"unexpected error when marshalling data")
				} else {
					g.Expect(err.Error()).Should(
						gomega.Equal(tc.expectedError.Error()),
						"unexpected error when marshalling data")
				}
			}

			g.Expect(string(buf)).To(gomega.Equal(tc.iniString),
				"marshalled config does not match")
		})
	}
}

func TestUnmarshalINI(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	deprecatedTestCases := []codecTestCase{
		{
			testName: "Global server is deprecated",
			iniString: `
			[Global]
			server = "deprecated"
			`,
			expectedError:    errDeprecated("Global", "server"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global datacenter is deprecated",
			iniString: `
			[Global]
			datacenter = "deprecated"
			`,
			expectedError:    errDeprecated("Global", "datacenter"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{

			testName: "Global datastore is deprecated",
			iniString: `
			[Global]
			datastore = "deprecated"
			`,
			expectedError:    errDeprecated("Global", "datastore"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global working-dir is deprecated",
			iniString: `
			[Global]
			working-dir = "deprecated"
			`,
			expectedError:    errDeprecated("Global", "working-dir"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global vm-name is deprecated",
			iniString: `
			[Global]
			vm-name = "deprecated"
			`,
			expectedError:    errDeprecated("Global", "vm-name"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global vm-uuid is deprecated",
			iniString: `
			[Global]
			vm-uuid = "deprecated"
			`,
			expectedError:    errDeprecated("Global", "vm-uuid"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
	}

	testcases := []codecTestCase{
		{
			testName: "Username and password in global section",
			iniString: `
		[Global]
		user = "user"
		password = "password"
		datacenters = "us-west"
		cluster-id = "cluster-namespace/cluster-name"

		[VirtualCenter "0.0.0.0"]

		[Workspace]
		server = "0.0.0.0"
		datacenter = "us-west"
		folder = "kubernetes"
		default-datastore = "default"
		`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
					ClusterID:   "cluster-namespace/cluster-name",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
					Datastore:  "default",
				},
			},
		},
		{
			testName: "Username and password in vCenter section",
			iniString: `
		[Global]
		port = "443"
		insecure-flag = true
		datacenters = "us-west"

		[VirtualCenter "0.0.0.0"]
		user = "user"
		password = "password"

		[Workspace]
		server = 0.0.0.0
		datacenter = "us-west"
		folder = "kubernetes"
		`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Port:        "443",
					Insecure:    true,
					Datacenters: "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Username: "user",
						Password: "password",
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "NetBIOS style AD username and password in vCenter section",
			iniString: `
		[Global]
		port = "443"
		insecure-flag = true
		datacenters = "us-west"

		[VirtualCenter "0.0.0.0"]
		user = "domain\\user"
		password = "password"

		[Workspace]
		server = 0.0.0.0
		datacenter = "us-west"
		folder = "kubernetes"
		`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Port:        "443",
					Insecure:    true,
					Datacenters: "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Username: "domain\\user",
						Password: "password",
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "SecretName and SecretNamespace",
			iniString: `
		[Global]
		secret-name = "vccreds"
		secret-namespace = "kube-system"
		datacenters = "us-west"

		[VirtualCenter "0.0.0.0"]

		[Workspace]
		server = "0.0.0.0"
		datacenter = "us-west"
		folder = "kubernetes"
		`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "SecretName and SecretNamespace with Username missing",
			iniString: `
		[Global]
		port = "443"
		insecure-flag = true
		datacenters = "us-west"
		secret-name = "vccreds"
		secret-namespace = "kube-system"

		[VirtualCenter "0.0.0.0"]
		password = "password"

		[Workspace]
		server = "0.0.0.0"
		datacenter = "us-west"
		folder = "kubernetes"
		`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Port:            "443",
					Insecure:        true,
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Password: "password",
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "Multiple virtual centers with different thumbprints",
			iniString: `
		[Global]
		user = "user"
		password = "password"
		datacenters = "us-west"

		[VirtualCenter "0.0.0.0"]
		thumbprint = "thumbprint:0"

		[VirtualCenter "no_thumbprint"]

		[VirtualCenter "1.1.1.1"]
		thumbprint = "thumbprint:1"

		[Workspace]
		server = "0.0.0.0"
		datacenter = "us-west"
		folder = "kubernetes"
		`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Thumbprint: "thumbprint:0",
					},
					"no_thumbprint": {},
					"1.1.1.1": {
						Thumbprint: "thumbprint:1",
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "Multiple vCenters using global CA cert",
			iniString: `
		[Global]
		datacenters = "us-west"
		secret-name = "vccreds"
		secret-namespace = "kube-system"
		ca-file = "/some/path/to/my/trusted/ca.pem"

		[VirtualCenter "0.0.0.0"]
		[VirtualCenter "1.1.1.1"]

		[Workspace]
		server = "0.0.0.0"
		datacenter = "us-west"
		folder = "kubernetes"
		`,
			configObj: v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Datacenters:     "us-west",
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					CAFile:          "/some/path/to/my/trusted/ca.pem",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {},
					"1.1.1.1": {},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
	}

	//nolint:gocritic
	testCases := append(
		testcases,
		deprecatedTestCases...,
	)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			var actualConfig v1alpha3.CPIConfig

			if err := actualConfig.UnmarshalINI(
				[]byte(tc.iniString),
				tc.unmarshalOptions...); err != nil {
				if tc.expectedError == nil {
					g.Expect(err).ShouldNot(
						gomega.HaveOccurred(),
						"unexpected error when unmarshalling data")
				} else {
					g.Expect(err.Error()).Should(
						gomega.Equal(tc.expectedError.Error()),
						"unexpected error when unmarshalling data")
				}
			}

			g.Expect(actualConfig).Should(
				gomega.Equal(tc.configObj),
				"actual config does not match expected config")
		})
	}
}

type passwordTestCase struct {
	testName         string
	iniEncodedString string
	expectedString   string
}

func TestPasswords(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	testCases := []passwordTestCase{
		{
			testName:         "password contains backslash",
			iniEncodedString: "pass\\\\word",
			expectedString:   "pass\\word",
		},
		{
			testName:         "password contains quotation mark",
			iniEncodedString: "pass\\\"word",
			expectedString:   "pass\"word",
		},
		{
			testName:         "password contains tab",
			iniEncodedString: "pass\\tword",
			expectedString:   "pass\tword",
		},
		{
			testName:         "password contains allowed characters for Microsoft Active Directory including Unicode",
			iniEncodedString: "0123456789abczABCZ~!@#$%^&*_-+=`|\\\\(){}[]:;\\\"'<>,.?/€Пассворд密码🌟",
			expectedString:   "0123456789abczABCZ~!@#$%^&*_-+=`|\\(){}[]:;\"'<>,.?/€Пассворд密码🌟",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			var actualConfig v1alpha3.CPIConfig

			iniString := `
			[Global]
			port = "443"
			insecure-flag = true
			datacenters = "us-west"

			[VirtualCenter "0.0.0.0"]
			user = "user"
			password = "%s"

			[Workspace]
			server = 0.0.0.0
			datacenter = "us-west"
			folder = "kubernetes"
`
			expectedConfig := v1alpha3.CPIConfig{
				Global: v1alpha3.CPIGlobalConfig{
					Port:        "443",
					Insecure:    true,
					Datacenters: "us-west",
				},
				VCenter: map[string]v1alpha3.CPIVCenterConfig{
					"0.0.0.0": {
						Username: "user",
						Password: tc.expectedString,
					},
				},
				Workspace: v1alpha3.CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			}

			err := actualConfig.UnmarshalINI([]byte(fmt.Sprintf(iniString, tc.iniEncodedString)))
			g.Expect(err).ToNot(gomega.HaveOccurred())

			g.Expect(actualConfig).Should(
				gomega.Equal(expectedConfig),
				"actual config does not match expected config")
		})
	}
}
