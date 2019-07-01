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

package context

import (
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/template"
)

// GovmomiContext is a context that adds govmomi objects to a MachineContext
type GovmomiContext struct {
	*context.MachineContext
	Src        *object.VirtualMachine
	Folder     *object.Folder
	Datastore  *object.Datastore
	Host       *object.HostSystem
	Datacenter *object.Datacenter
	Pool       *object.ResourcePool
}

// NewGovmomiContext creates a new GovmomiContext from a MachineContext
func NewGovmomiContext(ctx *context.MachineContext) (*GovmomiContext, error) {
	src, err := template.FindTemplate(ctx, ctx.MachineConfig.MachineSpec.VMTemplate)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find template: %s", ctx.MachineConfig.MachineSpec.VMTemplate)
	}

	host, err := src.HostSystem(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get HostSystem")
	}

	datastore, err := ctx.Session.Finder.DatastoreOrDefault(ctx, ctx.MachineConfig.MachineSpec.Datastore)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Datastore")
	}

	ctx.Logger.V(2).Info("attempting to get host resource pool")
	pool, err := host.ResourcePool(ctx.Context)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get host resource pool ")
	}

	dc, err := ctx.Session.Finder.DefaultDatacenter(ctx.Context)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get datacenter")
	}

	vmCtx := GovmomiContext{
		MachineContext: ctx,
		Datastore:      datastore,
		Pool:           pool,
		Datacenter:     dc,
		Host:           host,
		Src:            src,
	}
	return &vmCtx, nil
}

// GetVirtualMachineMO is a convenience method that wraps fetching the VirtualMachine
// MO from its higher-level object.
func (ctx *GovmomiContext) GetVirtualMachineMO() (*mo.VirtualMachine, error) {
	var props mo.VirtualMachine
	if err := ctx.Src.Properties(ctx.Context, ctx.Src.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}
