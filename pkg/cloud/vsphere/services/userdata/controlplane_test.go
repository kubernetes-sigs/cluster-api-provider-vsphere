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

package userdata

import "testing"

func TestTemplateYAMLIndent(t *testing.T) {
	testcases := []struct {
		name     string
		input    string
		indent   int
		expected string
	}{
		{
			name:     "simple case",
			input:    "hello\n  world",
			indent:   2,
			expected: "  hello\n    world",
		},
		{
			name:     "more indent",
			input:    "  some extra:\n    indenting\n",
			indent:   4,
			expected: "      some extra:\n        indenting\n    ",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			out := templateYAMLIndent(tc.indent, tc.input)
			if out != tc.expected {
				t.Fatalf("\nout:\n%+q\nexpected:\n%+q\n", out, tc.expected)
			}
		})
	}

}

func Test_CloudConfig(t *testing.T) {
	testcases := []struct {
		name     string
		input    *CloudConfigInput
		userdata string
		err      error
	}{
		{
			name: "standard cloud config",
			input: &CloudConfigInput{
				User:         "admin",
				Password:     "so_secure",
				Server:       "10.0.0.1",
				Datacenter:   "myprivatecloud",
				ResourcePool: "deadpool",
				Folder:       "vms",
				Datastore:    "infinite-data",
				Network:      "connected",
			},
			userdata: `[Global]
insecure-flag = "1" # set to 1 if the vCenter uses a self-signed cert
datacenters = "myprivatecloud"

[VirtualCenter "10.0.0.1"]
user = "admin"
password = "so_secure"

[Workspace]
server = "10.0.0.1"
datacenter = "myprivatecloud"
folder = "vms"
default-datastore = "infinite-data"
resourcepool-path = "deadpool"

[Disk]
scsicontrollertype = pvscsi

[Network]
public-network = "connected"
`,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			userdata, err := NewCloudConfig(testcase.input)
			if err != nil {
				t.Fatalf("error getting cloud config user data: %q", err)
			}

			if userdata != testcase.userdata {
				t.Logf("actual user data: %q", userdata)
				t.Logf("expected user data: %q", testcase.userdata)
				t.Error("unexpected user data")
			}
		})
	}
}
