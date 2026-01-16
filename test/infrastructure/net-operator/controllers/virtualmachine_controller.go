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

package controllers

import (
	"context"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	conversionclient "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/client"
)

const (
	// VMKickTimeAnnotation is an annotation used to trigger a reconcile to a VM.
	VMKickTimeAnnotation = "vcsim.fake.infrastructure.cluster.x-k8s.io/kick-time"
)

type VirtualMachineReconciler struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachines,verbs=get;list;watch;update;patch;delete

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *VirtualMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileTime := time.Now()

	// Fetch the VirtualMachine instance
	virtualMachine := &vmoprvhub.VirtualMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, virtualMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if virtualMachine.Status.PowerState == vmoprvhub.VirtualMachinePowerStateOn && virtualMachine.Status.Network != nil && virtualMachine.Status.Network.PrimaryIP4 != "" {
		return ctrl.Result{}, nil
	}

	// try to get the VirtualMachine to get ready as fast as possible by kicking reconcile.

	// give up after 15m (something is broken)
	if time.Since(virtualMachine.CreationTimestamp.Time) > 15*time.Minute {
		return ctrl.Result{}, nil
	}

	// add an annotation to the VirtualMachine 30s after creation, keep updating the annotation every 30s.
	kickTime := virtualMachine.CreationTimestamp.Time.Add(30 * time.Second)
	if t, ok := virtualMachine.GetAnnotations()[VMKickTimeAnnotation]; ok {
		var err error
		kickTime, err = time.Parse(time.RFC3339, t)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to parse date from %s annotation on %s", VMKickTimeAnnotation, klog.KObj(virtualMachine))
		}
	}

	if reconcileTime.Before(kickTime) {
		nextCheckAfter := kickTime.Sub(reconcileTime)
		return ctrl.Result{RequeueAfter: nextCheckAfter}, nil
	}

	o := virtualMachine.DeepCopy()
	annotations := virtualMachine.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[VMKickTimeAnnotation] = reconcileTime.Add(15 * time.Second).Format(time.RFC3339)

	virtualMachine.SetAnnotations(annotations)
	if err := r.Client.Patch(ctx, virtualMachine, client.MergeFrom(o)); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to patch VirtualMachine %s", klog.KObj(virtualMachine))
	}

	log.Info("Triggering VirtualMachine reconciliation to speed up initial provisioning")

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *VirtualMachineReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) error {
	// NOTE: use vm-operator native types for watches (the reconciler uses the internal hub version).
	vm, err := conversionclient.WatchObject(r.Client, &vmoprvhub.VirtualMachine{})
	if err != nil {
		return errors.Wrap(err, "failed to create watch object for VirtualMachines")
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(vm).
		WithOptions(options).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}
