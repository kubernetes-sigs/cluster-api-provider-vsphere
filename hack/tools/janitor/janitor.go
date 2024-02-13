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

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	govmomicluster "github.com/vmware/govmomi/vapi/cluster"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newJanitor(vSphereClients *vSphereClients, ipamClient client.Client, maxAge time.Duration, ipamNamespace string, dryRun bool) *janitor {
	return &janitor{
		dryRun:          dryRun,
		ipamClient:      ipamClient,
		ipamNamespace:   ipamNamespace,
		maxCreationDate: time.Now().Add(-maxAge),
		vSphereClients:  vSphereClients,
	}
}

type janitor struct {
	dryRun          bool
	ipamClient      client.Client
	ipamNamespace   string
	maxCreationDate time.Time
	vSphereClients  *vSphereClients
}

type virtualMachine struct {
	managedObject mo.VirtualMachine
	object        *object.VirtualMachine
}

// deleteVSphereVMs deletes all VSphereVMs in a given folder in vSphere if their creation
// timestamp is before the janitor's configured maxCreationDate.
func (s *janitor) deleteVSphereVMs(ctx context.Context, folder string) error {
	log := ctrl.LoggerFrom(ctx).WithName("vSphereVMs").WithValues("folder", folder)
	ctx = ctrl.LoggerInto(ctx, log)

	if folder == "" {
		return fmt.Errorf("cannot use empty string as folder")
	}

	log.Info("Deleting vSphere VMs in folder")

	// List all virtual machines inside the folder.
	finder := find.NewFinder(s.vSphereClients.Vim, false)
	managedObjects, err := finder.ManagedObjectListChildren(ctx, folder+"/...", "VirtualMachine")
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

	// Filter out vms we don't have to cleanup depending on s.maxCreationDate.
	for _, managedObjectVM := range managedObjectVMs {
		if managedObjectVM.Summary.Config.Template {
			// Skip templates for deletion.
			continue
		}
		if managedObjectVM.Config.CreateDate.After(s.maxCreationDate) {
			// Ignore vms created after maxCreationDate
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
		if dryRun {
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

func waitForTasksFinished(ctx context.Context, tasks []*object.Task, ignoreErrors bool) error {
	for _, t := range tasks {
		if err := t.WaitEx(ctx); !ignoreErrors && err != nil {
			return err
		}
	}
	return nil
}

func (s *janitor) deleteIPAddressClaims(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx).WithName("IPAddressClaims")
	ctrl.LoggerInto(ctx, log)
	log.Info("Deleting IPAddressClaims")

	// List all existing IPAddressClaims
	ipAddressClaims := &ipamv1.IPAddressClaimList{}
	if err := s.ipamClient.List(ctx, ipAddressClaims,
		client.InNamespace(s.ipamNamespace),
	); err != nil {
		return err
	}

	errList := []error{}

	for _, ipAddressClaim := range ipAddressClaims.Items {
		ipAddressClaim := ipAddressClaim
		// Skip IPAddressClaims which got created after maxCreationDate.
		if ipAddressClaim.CreationTimestamp.After(s.maxCreationDate) {
			continue
		}

		log.Info("Deleting IPAddressClaim", "IPAddressClaim", klog.KObj(&ipAddressClaim))

		if s.dryRun {
			// Skipping actual deletion on dryRun.
			continue
		}

		if err := s.ipamClient.Delete(ctx, &ipAddressClaim); err != nil {
			errList = append(errList, err)
		}
	}

	return kerrors.NewAggregate(errList)
}

func (s *janitor) deleteVSphereClusterModules(ctx context.Context) error {
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
