/*
Copyright 2026 The Kubernetes Authors.

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

// Package replicator implements template.ReplicaRequester with a
// ConfigMap-based lock (plain ConfigMaps work on any cluster; a
// coordination.k8s.io Lease isn't guaranteed to exist) and an async
// govmomi clone task to do the actual copy.
package replicator

import (
	"context"
	"crypto/sha1" //nolint:gosec // used only to derive a short, stable lock name
	"encoding/hex"
	"fmt"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/vmware/govmomi/fault"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/template"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

type Replicator struct {
	Client client.Client

	// Namespace for the lock ConfigMap. Always the manager's own, not the
	// calling VSphereVM's: workload clusters live in separate namespaces,
	// but a given template+datastore pair should only ever have one lock.
	Namespace string
}

var _ template.ReplicaRequester = &Replicator{}

const (
	// claimTimeout is how long a "claiming" placeholder (see tryClaim) is
	// honored before another caller can steal it. Covers a crash between
	// claiming the work and recording the real task ID.
	claimTimeout = 2 * time.Minute

	taskPlaceholderClaiming = "claiming"

	annotationTask      = "capv.infrastructure.cluster.x-k8s.io/replica-task"
	annotationClaimedAt = "capv.infrastructure.cluster.x-k8s.io/replica-claimed-at"
)

// RequestReplica implements template.ReplicaRequester. Non-blocking: a
// single round trip per call, whether that means checking on an in-flight
// clone or starting a new one. A failed attempt just releases the lock; the
// next call for this template and datastore, from whatever Machine makes
// it, starts over cleanly.
func (r *Replicator) RequestReplica(ctx context.Context, sess *session.Session, templateName string, master *object.VirtualMachine, pool *object.ResourcePool, datastore *object.Datastore) error {
	name := lockName(templateName, datastore.Reference())

	lock, err := r.getOrCreateLock(ctx, name)
	if err != nil {
		return pkgerrors.Wrap(err, "unable to get or create template replica lock")
	}
	if lock == nil {
		return nil // lost the create race; whoever won will drive it forward
	}

	return r.startOrCheckClone(ctx, sess, master, pool, datastore, lock)
}

// ReleaseIfDone cleans up a lock left behind once the Machine that called
// RequestReplica never comes back to notice the task finished. Only
// deletes when it can confirm the lock is no longer needed.
func (r *Replicator) ReleaseIfDone(ctx context.Context, sess *session.Session, templateName string, datastore *object.Datastore) {
	log := ctrl.LoggerFrom(ctx)

	name := lockName(templateName, datastore.Reference())

	var lock corev1.ConfigMap
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: name}, &lock); err != nil {
		return
	}

	taskID := lock.Annotations[annotationTask]
	if taskID == "" || taskID == taskPlaceholderClaiming {
		return // claim in progress elsewhere; don't act on a guess
	}

	state, _, err := r.checkTask(ctx, sess, taskID)
	if err != nil {
		return // transient read failure; nothing to clean up for certain yet
	}
	if state == types.TaskInfoStateRunning || state == types.TaskInfoStateQueued {
		return
	}

	if err := r.releaseLock(ctx, &lock); err != nil {
		log.V(4).Info("best-effort template replica lock cleanup failed", "lock", name, "error", err)
	}
}

// lockName deterministically names the lock for copying templateName onto
// datastore, so concurrent requests for the same pair contend on the same
// object. Keyed by name rather than a specific VM's MoRef: any existing
// copy of the template works equally well as a clone source, and which one
// gets picked can change from call to call.
func lockName(templateName string, datastore types.ManagedObjectReference) string {
	h := sha1.New() //nolint:gosec // name generation only
	_, _ = fmt.Fprintf(h, "%s/%s", templateName, datastore.Value)
	return "capv-tpl-replica-" + hex.EncodeToString(h.Sum(nil))[:20]
}

// getOrCreateLock returns the lock ConfigMap for name, creating it if
// needed. A nil lock with no error means creation raced with another caller
// who won.
func (r *Replicator) getOrCreateLock(ctx context.Context, name string) (*corev1.ConfigMap, error) {
	var lock corev1.ConfigMap
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: name}, &lock)
	switch {
	case apierrors.IsNotFound(err):
		return r.createLock(ctx, name)
	case err != nil:
		return nil, err
	}
	return &lock, nil
}

func (r *Replicator) createLock(ctx context.Context, name string) (*corev1.ConfigMap, error) {
	lock := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.Namespace,
		},
	}
	touchLock(lock)

	if err := r.Client.Create(ctx, lock); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, nil // lost the race between our Get and Create
		}
		return nil, err
	}
	return lock, nil
}

// tryClaim atomically claims the right to start the next task, so at most
// one caller ever proceeds to call govmomi.
func (r *Replicator) tryClaim(ctx context.Context, lock *corev1.ConfigMap) (bool, error) {
	if existing := lock.Annotations[annotationTask]; existing != "" {
		if existing != taskPlaceholderClaiming {
			return false, nil // a real task is already tracked
		}
		if !claimExpired(lock) {
			return false, nil // someone else's claim is still fresh
		}
		// stale claim, likely from a crash before the task was started; steal it
	}

	lock.Annotations[annotationTask] = taskPlaceholderClaiming
	touchLock(lock)
	if err := r.Client.Update(ctx, lock); err != nil {
		if apierrors.IsConflict(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func touchLock(lock *corev1.ConfigMap) {
	if lock.Annotations == nil {
		lock.Annotations = map[string]string{}
	}
	lock.Annotations[annotationClaimedAt] = time.Now().UTC().Format(time.RFC3339)
}

func claimExpired(lock *corev1.ConfigMap) bool {
	claimedAt, err := time.Parse(time.RFC3339, lock.Annotations[annotationClaimedAt])
	if err != nil {
		return true
	}
	return time.Now().After(claimedAt.Add(claimTimeout))
}

func (r *Replicator) releaseLock(ctx context.Context, lock *corev1.ConfigMap) error {
	if err := r.Client.Delete(ctx, lock); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// startOrCheckClone checks on a clone task already recorded on lock, or
// starts a new one if none is in flight yet. The Template flag goes into
// the clone spec itself rather than a later Reconfigure call: once a VM is
// marked as a template, not everything about it can still be changed.
func (r *Replicator) startOrCheckClone(ctx context.Context, sess *session.Session, master *object.VirtualMachine, pool *object.ResourcePool, datastore *object.Datastore, lock *corev1.ConfigMap) error {
	log := ctrl.LoggerFrom(ctx)

	if taskID := lock.Annotations[annotationTask]; taskID != "" && taskID != taskPlaceholderClaiming {
		state, taskFault, err := r.checkTask(ctx, sess, taskID)
		if err != nil {
			return err
		}
		switch state {
		case types.TaskInfoStateSuccess:
			log.V(4).Info("template replica created", "template", master.InventoryPath)
			return r.releaseLock(ctx, lock)
		case types.TaskInfoStateError:
			// DuplicateName means the VM already exists: a lost task
			// reference to an already-succeeded clone looks the same on
			// retry, so treat it as success.
			var dupName *types.DuplicateName
			if _, ok := fault.As(taskFault, &dupName); ok {
				log.V(4).Info("template replica already exists at destination", "template", master.InventoryPath, "existing", dupName.Object.Value)
				return r.releaseLock(ctx, lock)
			}
			log.V(2).Info("template replica clone failed; will retry on the next request", "template", master.InventoryPath)
			return r.releaseLock(ctx, lock)
		default:
			return nil // still running/queued
		}
	}

	claimed, err := r.tryClaim(ctx, lock)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	task, err := r.startClone(ctx, sess, master, pool, datastore)
	if err != nil {
		log.V(2).Info("unable to start template replica clone; will retry on the next request", "template", master.InventoryPath, "error", err)
		return r.releaseLock(ctx, lock)
	}
	lock.Annotations[annotationTask] = task.Reference().Value
	touchLock(lock)
	return r.Client.Update(ctx, lock)
}

func (r *Replicator) startClone(ctx context.Context, sess *session.Session, master *object.VirtualMachine, pool *object.ResourcePool, datastore *object.Datastore) (*object.Task, error) {
	var moMaster mo.VirtualMachine
	if err := master.Properties(ctx, master.Reference(), []string{"name"}, &moMaster); err != nil {
		return nil, pkgerrors.Wrap(err, "unable to read master template properties")
	}

	cluster, err := pool.Owner(ctx)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "unable to determine owning cluster for resource pool")
	}
	// NewComputeResource, not *ClusterComputeResource, so this also works
	// for a resource pool owned by a standalone host.
	computeResource := object.NewComputeResource(pool.Client(), cluster.Reference())
	hosts, err := computeResource.Hosts(ctx)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "unable to list hosts for owning cluster")
	}

	// Needs a host that actually mounts the target datastore, not just any
	// host in the cluster, or placement breaks even with Datastore set.
	dsRef := datastore.Reference()
	var moDS mo.Datastore
	if err := datastore.Properties(ctx, dsRef, []string{"host"}, &moDS); err != nil {
		return nil, pkgerrors.Wrap(err, "unable to read hosts accessible to target datastore")
	}
	dsHosts := make(map[types.ManagedObjectReference]bool, len(moDS.Host))
	for _, h := range moDS.Host {
		dsHosts[h.Key] = true
	}
	var hostRef *types.ManagedObjectReference
	for _, h := range hosts {
		if ref := h.Reference(); dsHosts[ref] {
			hostRef = &ref
			break
		}
	}
	if hostRef == nil {
		return nil, fmt.Errorf("datastore %q is not accessible from any host in cluster %q", dsRef.Value, cluster.Reference().Value)
	}

	// The replica keeps the master's name (that's how FindTemplate finds it
	// later), which means it needs its own subfolder per datastore to avoid
	// name collisions.
	defaultFolder, err := sess.Finder.FolderOrDefault(ctx, "")
	if err != nil {
		return nil, pkgerrors.Wrap(err, "unable to resolve default VM folder for template replica")
	}
	folder, err := ensureReplicaFolder(ctx, defaultFolder, dsRef.Value)
	if err != nil {
		return nil, err
	}

	poolRef := pool.Reference()
	spec := types.VirtualMachineCloneSpec{
		Location: types.VirtualMachineRelocateSpec{
			Datastore: &dsRef,
			Pool:      &poolRef,
			Host:      hostRef,
		},
		Template: true,
		PowerOn:  false,
	}

	task, err := master.Clone(ctx, folder, moMaster.Name, spec)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "unable to start template replica clone")
	}
	return task, nil
}

// replicaFolderName is the shared parent folder for all CAPV-managed
// template replicas; each datastore gets its own subfolder under it.
const replicaFolderName = "capv-template-replicas"

// ensureReplicaFolder returns the per-datastore replica folder under root,
// creating the shared parent and the datastore-specific child if either is
// missing.
func ensureReplicaFolder(ctx context.Context, root *object.Folder, datastoreDir string) (*object.Folder, error) {
	parent, err := ensureChildFolder(ctx, root, replicaFolderName)
	if err != nil {
		return nil, err
	}
	return ensureChildFolder(ctx, parent, datastoreDir)
}

func ensureChildFolder(ctx context.Context, parent *object.Folder, name string) (*object.Folder, error) {
	if existing, err := findChildFolder(ctx, parent, name); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	folder, err := parent.CreateFolder(ctx, name)
	if err != nil {
		// Probably lost a race to create it; check again before giving up.
		if existing, findErr := findChildFolder(ctx, parent, name); findErr == nil && existing != nil {
			return existing, nil
		}
		return nil, pkgerrors.Wrapf(err, "unable to create %q folder", name)
	}
	return folder, nil
}

func findChildFolder(ctx context.Context, parent *object.Folder, name string) (*object.Folder, error) {
	children, err := parent.Children(ctx)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "unable to list children of parent folder")
	}
	for _, child := range children {
		folder, ok := child.(*object.Folder)
		if !ok {
			continue
		}
		var moFolder mo.Folder
		if err := folder.Properties(ctx, folder.Reference(), []string{"name"}, &moFolder); err != nil {
			continue
		}
		if moFolder.Name == name {
			return folder, nil
		}
	}
	return nil, nil
}

// checkTask does a single, non-blocking read of a task's state and fault. A
// task vCenter no longer remembers is reported as TaskInfoStateError rather
// than a propagated error, so callers retry fresh instead of getting stuck.
func (r *Replicator) checkTask(ctx context.Context, sess *session.Session, taskMoRefValue string) (types.TaskInfoState, types.BaseMethodFault, error) {
	ref := types.ManagedObjectReference{Type: "Task", Value: taskMoRefValue}
	var moTask mo.Task
	if err := property.DefaultCollector(sess.Client.Client).RetrieveOne(ctx, ref, []string{"info"}, &moTask); err != nil {
		if fault.Is(err, &types.ManagedObjectNotFound{}) {
			return types.TaskInfoStateError, nil, nil
		}
		return "", nil, pkgerrors.Wrapf(err, "unable to read status of task %q", taskMoRefValue)
	}
	if moTask.Info.Error != nil {
		return types.TaskInfoStateError, moTask.Info.Error.Fault, nil
	}
	return moTask.Info.State, nil, nil
}
