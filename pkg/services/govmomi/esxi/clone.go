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

package esxi

import (
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// Clone kicks off a clone operation on ESXi to create a new virtual machine.
func Clone(ctx *context.VMContext, bootstrapData []byte) error {
	return errors.New("temporarily disabled esxi support")
}
