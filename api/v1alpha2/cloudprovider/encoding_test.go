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

package cloudprovider_test

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2/cloudprovider"
)

var unmarshalWarnAsFatal = []cloudprovider.UnmarshalINIOptionFunc{cloudprovider.WarnAsFatal}

func errDeprecated(section, key string) error {
	return errors.Errorf("warning:\ncan't store data at section \"%s\", variable \"%s\"\n", section, key)
}

type codecTestCase struct {
	testName         string
	iniString        string
	configObj        cloudprovider.Config
	expectedError    error
	unmarshalOptions []cloudprovider.UnmarshalINIOptionFunc
}

func TestMarshalINI(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	testcases := []codecTestCase{
		{
			testName: "Username and password in global section",
			iniString: `[Global]
user = user
password = password
datacenters = us-west
cluster-id = cluster-namespace/cluster-name

[VirtualCenter "0.0.0.0"]

[Workspace]
server = 0.0.0.0
datacenter = us-west
folder = kubernetes
default-datastore = default

`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
					ClusterID:   "cluster-namespace/cluster-name",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: cloudprovider.WorkspaceConfig{
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
port = 443
datacenters = us-west

[VirtualCenter "0.0.0.0"]
user = user
password = password

[Workspace]
server = 0.0.0.0
datacenter = us-west
folder = kubernetes

`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Port:        "443",
					Insecure:    true,
					Datacenters: "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {
						Username: "user",
						Password: "password",
					},
				},
				Workspace: cloudprovider.WorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "SecretName and SecretNamespace",
			iniString: `[Global]
secret-name = vccreds
secret-namespace = kube-system
datacenters = us-west

[VirtualCenter "0.0.0.0"]

[Workspace]
server = 0.0.0.0
datacenter = us-west
folder = kubernetes

`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: cloudprovider.WorkspaceConfig{
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
secret-name = vccreds
secret-namespace = kube-system
port = 443
datacenters = us-west

[VirtualCenter "0.0.0.0"]
password = password

[Workspace]
server = 0.0.0.0
datacenter = us-west
folder = kubernetes

`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Port:            "443",
					Insecure:        true,
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {
						Password: "password",
					},
				},
				Workspace: cloudprovider.WorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "Multiple virtual centers with different thumbprints",
			iniString: `[Global]
user = user
password = password
datacenters = us-west

[VirtualCenter "0.0.0.0"]
thumbprint = thumbprint:0

[VirtualCenter "1.1.1.1"]
thumbprint = thumbprint:1

[VirtualCenter "no_thumbprint"]

[Workspace]
server = 0.0.0.0
datacenter = us-west
folder = kubernetes

`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {
						Thumbprint: "thumbprint:0",
					},
					"no_thumbprint": {},
					"1.1.1.1": {
						Thumbprint: "thumbprint:1",
					},
				},
				Workspace: cloudprovider.WorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
		{
			testName: "Multiple vCenters using global CA cert",
			iniString: `[Global]
secret-name = vccreds
secret-namespace = kube-system
ca-file = /some/path/to/my/trusted/ca.pem
datacenters = us-west

[VirtualCenter "0.0.0.0"]

[VirtualCenter "1.1.1.1"]

[Workspace]
server = 0.0.0.0
datacenter = us-west
folder = kubernetes

`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Datacenters:     "us-west",
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					CAFile:          "/some/path/to/my/trusted/ca.pem",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {},
					"1.1.1.1": {},
				},
				Workspace: cloudprovider.WorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
				ProviderConfig: cloudprovider.ProviderConfig{
					Cloud: cloudprovider.CloudConfig{
						ControllerImage: "test",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
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
			server = deprecated
			`,
			expectedError:    errDeprecated("Global", "server"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global datacenter is deprecated",
			iniString: `
			[Global]
			datacenter = deprecated
			`,
			expectedError:    errDeprecated("Global", "datacenter"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{

			testName: "Global datastore is deprecated",
			iniString: `
			[Global]
			datastore = deprecated
			`,
			expectedError:    errDeprecated("Global", "datastore"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global working-dir is deprecated",
			iniString: `
			[Global]
			working-dir = deprecated
			`,
			expectedError:    errDeprecated("Global", "working-dir"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global vm-name is deprecated",
			iniString: `
			[Global]
			vm-name = deprecated
			`,
			expectedError:    errDeprecated("Global", "vm-name"),
			unmarshalOptions: unmarshalWarnAsFatal,
		},
		{
			testName: "Global vm-uuid is deprecated",
			iniString: `
			[Global]
			vm-uuid = deprecated
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
		user = user
		password = password
		datacenters = us-west
		cluster-id = cluster-namespace/cluster-name

		[VirtualCenter "0.0.0.0"]

		[Workspace]
		server = 0.0.0.0
		datacenter = us-west
		folder = kubernetes
		default-datastore = default
		`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
					ClusterID:   "cluster-namespace/cluster-name",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: cloudprovider.WorkspaceConfig{
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
		port = 443
		insecure-flag = true
		datacenters = us-west

		[VirtualCenter "0.0.0.0"]
		user = user
		password = password

		[Workspace]
		server = 0.0.0.0
		datacenter = us-west
		folder = kubernetes
		`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Port:        "443",
					Insecure:    true,
					Datacenters: "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {
						Username: "user",
						Password: "password",
					},
				},
				Workspace: cloudprovider.WorkspaceConfig{
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
		datacenters = us-west

		[VirtualCenter "0.0.0.0"]

		[Workspace]
		server = 0.0.0.0
		datacenter = us-west
		folder = kubernetes
		`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: cloudprovider.WorkspaceConfig{
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
		port = 443
		insecure-flag = true
		datacenters = us-west
		secret-name = "vccreds"
		secret-namespace = "kube-system"

		[VirtualCenter "0.0.0.0"]
		password = password

		[Workspace]
		server = 0.0.0.0
		datacenter = us-west
		folder = kubernetes
		`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Port:            "443",
					Insecure:        true,
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {
						Password: "password",
					},
				},
				Workspace: cloudprovider.WorkspaceConfig{
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
		user = user
		password = password
		datacenters = us-west

		[VirtualCenter "0.0.0.0"]
		thumbprint = thumbprint:0

		[VirtualCenter "no_thumbprint"]

		[VirtualCenter "1.1.1.1"]
		thumbprint = thumbprint:1

		[Workspace]
		server = 0.0.0.0
		datacenter = us-west
		folder = kubernetes
		`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {
						Thumbprint: "thumbprint:0",
					},
					"no_thumbprint": {},
					"1.1.1.1": {
						Thumbprint: "thumbprint:1",
					},
				},
				Workspace: cloudprovider.WorkspaceConfig{
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
		ca-file = /some/path/to/my/trusted/ca.pem

		[VirtualCenter "0.0.0.0"]
		[VirtualCenter "1.1.1.1"]

		[Workspace]
		server = 0.0.0.0
		datacenter = us-west
		folder = kubernetes
		`,
			configObj: cloudprovider.Config{
				Global: cloudprovider.GlobalConfig{
					Datacenters:     "us-west",
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					CAFile:          "/some/path/to/my/trusted/ca.pem",
				},
				VCenter: map[string]cloudprovider.VCenterConfig{
					"0.0.0.0": {},
					"1.1.1.1": {},
				},
				Workspace: cloudprovider.WorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
			},
		},
	}

	testCases := append(
		testcases,
		deprecatedTestCases...,
	)

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			var actualConfig cloudprovider.Config

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
