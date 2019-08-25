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

package govmomi

import (
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/esxi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/vcenter"
)

const (
	// nodeRole is the label assigned to every node in the cluster.
	nodeRole = "node-role.kubernetes.io/node="

	// the Kubernetes cloud provider to use
	cloudProvider = "vsphere"

	// the cloud config path read by the cloud provider
	cloudConfigPath = "/etc/kubernetes/vsphere.conf"
)

// Create creates a new machine.
func Create(ctx *context.MachineContext, bootstrapData []byte) error {
	// Check to see if the VM exists first since no error is returned if the VM
	// does not exist, only when there's an error checking or when the op should
	// be requeued, like when the VM has an in-flight task.
	vm, err := lookupVM(ctx)
	if err != nil {
		return err
	}
	if vm != nil {
		return errors.Errorf("vm already exists for %q", ctx)
	}

	if ctx.Session.IsVC() {
		return vcenter.Clone(ctx, bootstrapData)
	}
	return esxi.Clone(ctx, bootstrapData)
}
