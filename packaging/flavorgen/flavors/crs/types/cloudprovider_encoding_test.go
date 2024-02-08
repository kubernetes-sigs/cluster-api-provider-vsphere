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

package types

import (
	"testing"

	"github.com/onsi/gomega"
)

type codecTestCase struct {
	testName      string
	iniString     string
	configObj     CPIConfig
	expectedError error
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
			configObj: CPIConfig{
				Global: CPIGlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
					ClusterID:   "cluster-namespace/cluster-name",
				},
				VCenter: map[string]CPIVCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: CPIWorkspaceConfig{
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
			configObj: CPIConfig{
				Global: CPIGlobalConfig{
					Port:        "443",
					Insecure:    true,
					Datacenters: "us-west",
				},
				VCenter: map[string]CPIVCenterConfig{
					"0.0.0.0": {
						Username: "user",
						Password: "password",
					},
				},
				Workspace: CPIWorkspaceConfig{
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
			configObj: CPIConfig{
				Global: CPIGlobalConfig{
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]CPIVCenterConfig{
					"0.0.0.0": {},
				},
				Workspace: CPIWorkspaceConfig{
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
			configObj: CPIConfig{
				Global: CPIGlobalConfig{
					Port:            "443",
					Insecure:        true,
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					Datacenters:     "us-west",
				},
				VCenter: map[string]CPIVCenterConfig{
					"0.0.0.0": {
						Password: "password",
					},
				},
				Workspace: CPIWorkspaceConfig{
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
			configObj: CPIConfig{
				Global: CPIGlobalConfig{
					Username:    "user",
					Password:    "password",
					Datacenters: "us-west",
				},
				VCenter: map[string]CPIVCenterConfig{
					"0.0.0.0": {
						Thumbprint: "thumbprint:0",
					},
					"no_thumbprint": {},
					"1.1.1.1": {
						Thumbprint: "thumbprint:1",
					},
				},
				Workspace: CPIWorkspaceConfig{
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
			configObj: CPIConfig{
				Global: CPIGlobalConfig{
					Datacenters:     "us-west",
					SecretName:      "vccreds",
					SecretNamespace: "kube-system",
					CAFile:          "/some/path/to/my/trusted/ca.pem",
				},
				VCenter: map[string]CPIVCenterConfig{
					"0.0.0.0": {},
					"1.1.1.1": {},
				},
				Workspace: CPIWorkspaceConfig{
					Server:     "0.0.0.0",
					Datacenter: "us-west",
					Folder:     "kubernetes",
				},
				ProviderConfig: CPIProviderConfig{
					Cloud: &CPICloudConfig{
						ControllerImage: "test",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.testName, func(*testing.T) {
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
