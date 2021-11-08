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

package manager

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
)

func TestOptions_GetCredentials(t *testing.T) {
	g := NewWithT(t)
	contentFmt := `---
username: '%s'
password: '%s'
`
	tests := []struct {
		name                   string
		username, escapedUname string
		password, escapedPwd   string
	}{
		{
			name:         "username & password with no special characters",
			username:     "abcd",
			escapedUname: "abcd",
			password:     "password",
			escapedPwd:   "password",
		},
		{
			name:         "username with UPN ",
			username:     `vsphere.local\user`,
			escapedUname: `vsphere.local\user`,
			password:     `pass\word`,
			escapedPwd:   "pass\\word",
		},
	}

	for _, tt := range tests {
		// for linting reasons
		test := tt
		content := fmt.Sprintf(contentFmt, tt.username, tt.password)
		tmpFile, err := os.CreateTemp("", "creds")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(tmpFile.Name()) })

		if _, err := tmpFile.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatal(err)
		}

		t.Run(test.name, func(t *testing.T) {
			o := &Options{
				// needs an object ref to be present
				KubeConfig:      &rest.Config{},
				CredentialsFile: tmpFile.Name(),
			}
			o.defaults()

			g.Expect(o.Username).To(Equal(test.escapedUname))
			g.Expect(o.Password).To(Equal(test.escapedPwd))
		})
	}
}
