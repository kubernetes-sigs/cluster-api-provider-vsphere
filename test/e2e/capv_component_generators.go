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
	"encoding/base64"
	"fmt"
)

type credentialsGenerator struct{}

// GetName returns the name of the components being generated.
func (g credentialsGenerator) GetName() string {
	return "Cluster API Provider vSphere version: Bootstrap credentials"
}

// Manifests return the generated components and any possible error.
func (g credentialsGenerator) Manifests(ctx context.Context) ([]byte, error) {

	username64 := base64.StdEncoding.EncodeToString([]byte(vsphereUsername))
	password64 := base64.StdEncoding.EncodeToString([]byte(vspherePassword))

	return []byte(fmt.Sprintf(`---
apiVersion: v1
kind: Namespace
metadata:
  name: capv-system
---
apiVersion: v1
kind: Secret
metadata:
  name: capv-manager-bootstrap-credentials
  namespace: capv-system
type: Opaque
data:
  username: %s
  password: %s
`, username64, password64)), nil
}
