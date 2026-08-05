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

// Package template has tools for finding VM templates.
package template

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	pkgerrors "github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

// ReplicaRequester asynchronously clones master onto datastore when no copy
// of it exists there yet. Locality is per-datastore, not per-cluster (see
// vcenter.Clone). Implementations must not block: RequestReplica is called
// opportunistically and the copy just becomes visible on a later call.
type ReplicaRequester interface {
	RequestReplica(ctx context.Context, session *session.Session, templateName string, master *object.VirtualMachine, pool *object.ResourcePool, datastore *object.Datastore) error

	// ReleaseIfDone cleans up bookkeeping left behind by a prior
	// RequestReplica call once a local copy confirms it's ready, since a
	// VSphereVM only calls FindTemplate once. Must not block, and must be
	// safe to call with nothing to clean up.
	ReleaseIfDone(ctx context.Context, session *session.Session, templateName string, datastore *object.Datastore)
}

// FindTemplate finds a template by UUID or name. When TemplateAutoReplicate
// is enabled and preferredDatastore is given, a copy already on that
// datastore is preferred over a slow cross-datastore clone, and one is
// requested in the background if missing. Otherwise, behavior is unchanged
// from before this gate existed.
func FindTemplate(ctx context.Context, session *session.Session, templateID string, preferredPool *object.ResourcePool, preferredDatastore *object.Datastore, requester ReplicaRequester) (*object.VirtualMachine, error) {
	tpl, err := findTemplateByInstanceUUID(ctx, session, templateID)
	if err != nil {
		return nil, err
	}
	if tpl != nil {
		return tpl, nil
	}
	if preferredDatastore != nil && feature.Gates.Enabled(feature.TemplateAutoReplicate) {
		return findOrRequestLocalCopy(ctx, session, templateID, preferredPool, preferredDatastore, requester)
	}
	return findTemplateByName(ctx, session, templateID)
}

func findTemplateByInstanceUUID(ctx context.Context, session *session.Session, templateID string) (*object.VirtualMachine, error) {
	log := ctrl.LoggerFrom(ctx)

	if !isValidUUID(templateID) {
		return nil, nil
	}
	log.V(5).Info("Find template by instanceUUID", "instanceUUID", templateID)
	ref, err := session.FindByInstanceUUID(ctx, templateID)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "error querying template by instance UUID")
	}
	if ref != nil {
		return object.NewVirtualMachine(session.Client.Client, ref.Reference()), nil
	}
	return nil, nil
}

// findTemplateByName is the original lookup, exactly as it was before
// TemplateAutoReplicate existed: it errors if templateID resolves to more
// than one VM, with no notion of datastore locality.
func findTemplateByName(ctx context.Context, session *session.Session, templateID string) (*object.VirtualMachine, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(5).Info("Find template by name", "name", templateID)
	tpl, err := session.Finder.VirtualMachine(ctx, templateID)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "unable to find template by name %q", templateID)
	}
	return tpl, nil
}

// findOrRequestLocalCopy prefers a copy of templateID already on
// preferredDatastore, however it got there (placed by an admin or cloned
// earlier by CAPV). If none exists yet, a copy is requested in the
// background and any existing copy is returned so the caller can proceed
// without waiting for it.
func findOrRequestLocalCopy(ctx context.Context, session *session.Session, templateID string, preferredPool *object.ResourcePool, preferredDatastore *object.Datastore, requester ReplicaRequester) (*object.VirtualMachine, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(5).Info("Find template by name", "name", templateID)

	// Every VM sharing this name, not just one: a local copy is cloned with
	// the master's own name (see replicator.Replicator), so the
	// single-result Finder.VirtualMachine can't be used here.
	candidates, err := session.Finder.VirtualMachineList(ctx, templateID)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "unable to find template by name %q", templateID)
	}
	ambiguousErr := pkgerrors.Wrapf(fmt.Errorf("path '%s' resolves to multiple vms", templateID), "unable to find template by name %q", templateID)

	local := filterLocalTemplates(ctx, candidates, preferredDatastore)
	switch len(local) {
	case 1:
		if requester != nil {
			requester.ReleaseIfDone(ctx, session, templateID, preferredDatastore)
		}
		return local[0], nil
	case 0:
		master := candidates[0] // any existing copy works as the clone source
		requestReplica(ctx, log, requester, session, templateID, master, preferredPool, preferredDatastore)
		return master, nil
	default:
		return nil, ambiguousErr
	}
}

// requestReplica never returns an error: a failed or slow copy request must
// not affect the Machine being cloned right now, which already has a
// correct answer (master) to proceed with.
func requestReplica(ctx context.Context, log logr.Logger, requester ReplicaRequester, session *session.Session, templateName string, master *object.VirtualMachine, pool *object.ResourcePool, datastore *object.Datastore) {
	if requester == nil {
		log.V(4).Info("no local template copy available and no ReplicaRequester configured; skipping",
			"template", templateName, "datastore", datastore.InventoryPath)
		return
	}
	if err := requester.RequestReplica(ctx, session, templateName, master, pool, datastore); err != nil {
		log.V(2).Info("failed to request template copy",
			"template", templateName, "datastore", datastore.InventoryPath, "error", err)
	}
}

// filterLocalTemplates returns the subset of candidates that reside on
// datastore exactly (not just the same cluster; see ReplicaRequester).
// Returns nil if datastore is nil, same as no local candidates.
func filterLocalTemplates(ctx context.Context, candidates []*object.VirtualMachine, datastore *object.Datastore) []*object.VirtualMachine {
	log := ctrl.LoggerFrom(ctx)

	if datastore == nil {
		return nil
	}
	dsRef := datastore.Reference()

	var matches []*object.VirtualMachine
	for _, c := range candidates {
		on, err := vmOnDatastore(ctx, c, dsRef)
		if err != nil {
			log.V(4).Info("unable to read datastore property for candidate template; skipping", "template", c.InventoryPath, "error", err)
			continue
		}
		if on {
			matches = append(matches, c)
		}
	}
	return matches
}

// vmOnDatastore reports whether vm's config files reside on the datastore
// identified by dsRef.
func vmOnDatastore(ctx context.Context, vm *object.VirtualMachine, dsRef types.ManagedObjectReference) (bool, error) {
	var moVM mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"datastore"}, &moVM); err != nil {
		return false, err
	}
	for _, ds := range moVM.Datastore {
		if ds == dsRef {
			return true, nil
		}
	}
	return false, nil
}

func isValidUUID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}
