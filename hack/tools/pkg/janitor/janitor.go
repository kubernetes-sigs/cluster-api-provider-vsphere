/*
Copyright 2024 The Kubernetes Authors.

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

// Package janitor implements a janitor for vSphere.
package janitor

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/pkg/errors"
	cnstypes "github.com/vmware/govmomi/cns/types"
	"github.com/vmware/govmomi/object"
	govmomicluster "github.com/vmware/govmomi/vapi/cluster"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewJanitor creates a new Janitor.
func NewJanitor(vSphereClients *VSphereClients, dryRun bool) *Janitor {
	return &Janitor{
		dryRun:         dryRun,
		vSphereClients: vSphereClients,
	}
}

// Janitor implements a janitor for vSphere.
type Janitor struct {
	dryRun         bool
	vSphereClients *VSphereClients
}

type virtualMachine struct {
	managedObject mo.VirtualMachine
	object        *object.VirtualMachine
}

// boskosResourceLabel is used to identify volumes created in e2e tests.
// The value should contain the boskos resource name.
const boskosResourceLabel = "capv-e2e-test-boskos-resource"

// CleanupVSphere cleans up vSphere VMs, folders and resource pools.
func (s *Janitor) CleanupVSphere(ctx context.Context, folders, resourcePools, vmFolders []string, boskosResourceName string, skipClusterModule bool) error {
	errList := []error{}

	// Delete vms to cleanup folders and resource pools.
	for _, folder := range vmFolders {
		if err := s.deleteVSphereVMs(ctx, folder); err != nil {
			errList = append(errList, errors.Wrapf(err, "cleaning up vSphereVMs for folder %q", folder))
		}
	}
	if err := kerrors.NewAggregate(errList); err != nil {
		return errors.Wrap(err, "cleaning up vSphereVMs")
	}

	// Delete empty resource pools.
	for _, resourcePool := range resourcePools {
		if err := s.deleteObjectChildren(ctx, resourcePool, "ResourcePool"); err != nil {
			errList = append(errList, errors.Wrapf(err, "cleaning up empty resource pool children for resource pool %q", resourcePool))
		}
	}
	if err := kerrors.NewAggregate(errList); err != nil {
		return errors.Wrap(err, "cleaning up resource pools")
	}

	// Delete empty folders.
	for _, folder := range folders {
		if err := s.deleteObjectChildren(ctx, folder, "Folder"); err != nil {
			errList = append(errList, errors.Wrapf(err, "cleaning up empty folder children for folder %q", folder))
		}
	}
	if err := kerrors.NewAggregate(errList); err != nil {
		return errors.Wrap(err, "cleaning up folders")
	}

	// Delete CNS volumes.
	if err := s.DeleteCNSVolumes(ctx, boskosResourceName); err != nil {
		return errors.Wrap(err, "cleaning up volumes")
	}

	if skipClusterModule {
		return nil
	}

	// Delete empty cluster modules.
	if err := s.deleteVSphereClusterModules(ctx); err != nil {
		return errors.Wrap(err, "cleaning up vSphere cluster modules")
	}

	return nil
}

// deleteVSphereVMs deletes all VSphereVMs in a given folder in vSphere.
func (s *Janitor) deleteVSphereVMs(ctx context.Context, folder string) error {
	log := ctrl.LoggerFrom(ctx).WithName("vSphereVMs").WithValues("folder", folder)
	ctx = ctrl.LoggerInto(ctx, log)

	if folder == "" {
		return fmt.Errorf("cannot use empty string as folder")
	}

	log.Info("Deleting vSphere VMs in folder")

	// List all virtual machines inside the folder.
	managedObjects, err := s.vSphereClients.Finder.ManagedObjectListChildren(ctx, folder+"/...", "VirtualMachine")
	if err != nil {
		return err
	}

	if len(managedObjects) == 0 {
		return nil
	}

	// Retrieve information for all found virtual machines.
	managedObjectReferences := []types.ManagedObjectReference{}
	for _, obj := range managedObjects {
		managedObjectReferences = append(managedObjectReferences, obj.Object.Reference())
	}
	var managedObjectVMs []mo.VirtualMachine
	if err := s.vSphereClients.Govmomi.Retrieve(ctx, managedObjectReferences, []string{"config", "summary.runtime.powerState", "summary.config.template"}, &managedObjectVMs); err != nil {
		return err
	}

	vmsToDeleteAndPoweroff := []*virtualMachine{}
	vmsToDelete := []*virtualMachine{}

	// Figure out which VMs to delete and which to power off and delete.
	for _, managedObjectVM := range managedObjectVMs {
		if managedObjectVM.Summary.Config.Template {
			// Skip templates for deletion.
			continue
		}

		vm := &virtualMachine{
			managedObject: managedObjectVM,
			object:        object.NewVirtualMachine(s.vSphereClients.Vim, managedObjectVM.Reference()),
		}

		if vm.managedObject.Summary.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
			vmsToDeleteAndPoweroff = append(vmsToDeleteAndPoweroff, vm)
			continue
		}
		vmsToDelete = append(vmsToDelete, vm)
	}

	// PowerOff vms which are still running. Triggering PowerOff for a VM results in a task in vSphere.
	poweroffTasks := []*object.Task{}
	for _, vm := range vmsToDeleteAndPoweroff {
		log.Info("Powering off vm in vSphere", "vm", vm.managedObject.Config.Name)
		if s.dryRun {
			// Skipping actual PowerOff on dryRun.
			continue
		}
		task, err := vm.object.PowerOff(ctx)
		if err != nil {
			return err
		}
		log.Info("Created PowerOff task for VM", "vm", vm.managedObject.Config.Name, "task", task.Reference().Value)
		poweroffTasks = append(poweroffTasks, task)
	}
	// Wait for all PowerOff tasks to be finished. We intentionally ignore errors here
	// because the VM may already got into PowerOff state and log the errors only.
	// We are logging as best effort. If a machine did not successfully PowerOff, the
	// Destroy task below will result in an error.
	// xref govc: https://github.com/vmware/govmomi/blob/512c168/govc/vm/destroy.go#L94-L96
	if err := waitForTasksFinished(ctx, poweroffTasks, true); err != nil {
		log.Info("Ignoring error for PowerOff task", "err", err)
	}

	destroyTasks := []*object.Task{}
	for _, vm := range append(vmsToDeleteAndPoweroff, vmsToDelete...) {
		log.Info("Destroying vm in vSphere", "vm", vm.managedObject.Config.Name)
		if s.dryRun {
			// Skipping actual destroy on dryRun.
			continue
		}
		task, err := vm.object.Destroy(ctx)
		if err != nil {
			return err
		}
		log.Info("Created Destroy task for VM", "vm", vm.managedObject.Config.Name, "task", task.Reference().Value)
		destroyTasks = append(destroyTasks, task)
	}
	// Wait for all destroy tasks to succeed.
	if err := waitForTasksFinished(ctx, destroyTasks, false); err != nil {
		return errors.Wrap(err, "failed to wait for vm destroy task to finish")
	}

	return nil
}

// DeleteCNSVolumes deletes all volumes from tests.
func (s *Janitor) DeleteCNSVolumes(ctx context.Context, boskosResourceName string) error {
	log := ctrl.LoggerFrom(ctx).WithName("volumes")
	ctx = ctrl.LoggerInto(ctx, log)

	log.Info("Deleting volumes")

	type cnsVolumeToDelete struct {
		volumeID     cnstypes.CnsVolumeId
		pvcName      string
		pvcNamespace string
	}
	volumesToDelete := []cnsVolumeToDelete{}

	queryFilter := cnstypes.CnsQueryFilter{
		Labels: []types.KeyValue{
			{
				Key:   boskosResourceLabel,
				Value: boskosResourceName,
			},
		},
	}

	for {
		res, err := s.vSphereClients.CNS.QueryVolume(ctx, queryFilter)
		if err != nil {
			return err
		}

		for _, volume := range res.Volumes {
			var pvcMetadata *cnstypes.CnsKubernetesEntityMetadata
			for _, meta := range volume.Metadata.EntityMetadata {
				k8sMetadata, ok := meta.(*cnstypes.CnsKubernetesEntityMetadata)
				if !ok {
					continue
				}
				if k8sMetadata.EntityType != string(cnstypes.CnsKubernetesEntityTypePVC) {
					continue
				}
				pvcMetadata = k8sMetadata
			}

			if pvcMetadata == nil {
				// Ignoring non-PVC volumes.
				continue
			}

			var matchesBoskosResourcename bool
			// Check again that the volume has a matching label.
			for _, v := range pvcMetadata.Labels {
				if v.Key != boskosResourceLabel {
					continue
				}
				if v.Value != boskosResourceName {
					continue
				}
				matchesBoskosResourcename = true
			}

			// Ignore not matching volume.
			if !matchesBoskosResourcename {
				continue
			}

			volumesToDelete = append(volumesToDelete, cnsVolumeToDelete{
				volumeID:     volume.VolumeId,
				pvcName:      pvcMetadata.EntityName,
				pvcNamespace: pvcMetadata.Namespace,
			})
		}

		if res.Cursor.Offset == res.Cursor.TotalRecords || len(res.Volumes) == 0 {
			break
		}

		queryFilter.Cursor = &res.Cursor
	}

	if len(volumesToDelete) == 0 {
		log.Info("No CNS Volumes to delete")
		return nil
	}

	deleteTasks := []*object.Task{}
	for _, volume := range volumesToDelete {
		log := log.WithValues("volumeID", volume.volumeID, "PersistentVolumeClaim", klog.KRef(volume.pvcNamespace, volume.pvcName))

		log.Info("Deleting CNS Volume in vSphere")

		if s.dryRun {
			// Skipping actual delete on dryRun.
			continue
		}

		// Trigger deletion of the CNS Volume
		task, err := s.vSphereClients.CNS.DeleteVolume(ctx, []cnstypes.CnsVolumeId{volume.volumeID}, true)
		if err != nil {
			return errors.Wrap(err, "failed to create CNS Volume deletion task")
		}

		log.Info("Created CNS Volume deletion task", "task", task.Reference().Value)
		deleteTasks = append(deleteTasks, task)
	}

	// Wait for all delete tasks to succeed.
	if err := waitForTasksFinished(ctx, deleteTasks, false); err != nil {
		return errors.Wrap(err, "failed to wait for CNS Volume deletion tasks to finish")
	}

	return nil
}

// deleteObjectChildren deletes all child objects in a given object in vSphere if they don't
// contain any virtual machine.
// An object only gets deleted if:
// * it does not have any children of a different type
// If an object does not yet have a field, the janitor will add the field to it with the current timestamp as value.
func (s *Janitor) deleteObjectChildren(ctx context.Context, inventoryPath string, objectType string) error {
	if !slices.Contains([]string{"ResourcePool", "Folder"}, objectType) {
		return fmt.Errorf("deleteObjectChildren is not implemented for objectType %s", objectType)
	}

	if inventoryPath == "" {
		return fmt.Errorf("cannot use empty string to delete children of type %s", objectType)
	}

	log := ctrl.LoggerFrom(ctx).WithName(fmt.Sprintf("%sChildren", objectType)).WithValues(objectType, inventoryPath)
	ctx = ctrl.LoggerInto(ctx, log)

	log.Info("Deleting empty children")

	// Recursively list all objects of the given objectType below the inventoryPath.
	managedEntities, err := recursiveList(ctx, inventoryPath, s.vSphereClients.Govmomi, s.vSphereClients.Finder, s.vSphereClients.ViewManager, objectType)
	if err != nil {
		return err
	}

	// Build a map which notes if an object has children of a different type.
	// Later on we will use that information to not delete objects which have children.
	hasChildren := map[string]bool{}
	for _, e := range managedEntities {
		// Check if the object has children, because we only want to delete objects which have children of a different type.
		children, err := recursiveList(ctx, e.element.Path, s.vSphereClients.Govmomi, s.vSphereClients.Finder, s.vSphereClients.ViewManager)
		if err != nil {
			return err
		}
		// Mark e to have children, if there are children which are of a different type.
		for _, child := range children {
			if child.entity.Reference().Type == objectType {
				continue
			}
			hasChildren[e.element.Path] = true
			break
		}
	}

	objectsToDelete := []*managedElement{}

	// Filter elements and collect objects to destroy.
	for i := range managedEntities {
		managedEntity := managedEntities[i]

		// Filter out objects which have children.
		if hasChildren[managedEntity.element.Path] {
			log.Info("Skipping deletion of object: object has child objects of a different type", "inventoryPath", managedEntity.element.Path)
			continue
		}

		objectsToDelete = append(objectsToDelete, managedEntity)
	}

	// sort objects to delete so children are deleted before parents
	sort.Slice(objectsToDelete, func(i, j int) bool {
		a := objectsToDelete[i]
		b := objectsToDelete[j]

		return strings.Count(a.element.Path, "/") > strings.Count(b.element.Path, "/")
	})

	destroyTasks := []*object.Task{}
	for _, managedEntity := range objectsToDelete {
		log.Info("Destroying object in vSphere", objectType, managedEntity.element.Path)
		if s.dryRun {
			// Skipping actual destroy on dryRun.
			continue
		}

		task, err := object.NewCommon(s.vSphereClients.Vim, managedEntity.entity.Reference()).Destroy(ctx)
		if err != nil {
			return err
		}
		log.Info("Created Destroy task for object", objectType, managedEntity.element.Path, "task", task.Reference().Value)
		destroyTasks = append(destroyTasks, task)
	}
	// Wait for all destroy tasks to succeed.
	if err := waitForTasksFinished(ctx, destroyTasks, false); err != nil {
		return errors.Wrap(err, "failed to wait for object destroy task to finish")
	}

	return nil
}

func (s *Janitor) deleteVSphereClusterModules(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx).WithName("vSphere cluster modules")
	ctrl.LoggerInto(ctx, log)
	log.Info("Deleting vSphere cluster modules")

	manager := govmomicluster.NewManager(s.vSphereClients.Rest)

	// List all existing modules
	clusterModules, err := manager.ListModules(ctx)
	if err != nil {
		return err
	}

	errList := []error{}
	// Check for all modules if they refer members and delete them if they are empty.
	for _, clusterModule := range clusterModules {
		members, err := manager.ListModuleMembers(ctx, clusterModule.Module)
		if err != nil {
			errList = append(errList, err)
			continue
		}

		// Do not attempt to delete if the cluster module still refers virtual machines.
		if len(members) > 0 {
			continue
		}

		log.Info("Deleting empty vSphere cluster module", "clusterModule", clusterModule.Module)

		if s.dryRun {
			// Skipping actual deletion on dryRun.
			continue
		}

		if err := manager.DeleteModule(ctx, clusterModule.Module); err != nil {
			errList = append(errList, err)
		}
	}

	return kerrors.NewAggregate(errList)
}
