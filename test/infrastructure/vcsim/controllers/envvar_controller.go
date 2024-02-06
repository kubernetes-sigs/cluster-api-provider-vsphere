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

package controllers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	vcsimhelpers "sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

type EnvVarReconciler struct {
	Client         client.Client
	SupervisorMode bool

	PodIP   string
	sshKeys map[string]string
	lock    sync.RWMutex

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=envvars,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=envvars/status,verbs=get;update;patch

func (r *EnvVarReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the EnvVar instance
	envVar := &vcsimv1.EnvVar{}
	if err := r.Client.Get(ctx, req.NamespacedName, envVar); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the VCenterSimulator instance
	if envVar.Spec.VCenterSimulator == "" {
		return ctrl.Result{}, errors.New("Spec.VCenter cannot be empty")
	}

	vCenterSimulator := &vcsimv1.VCenterSimulator{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: envVar.Namespace,
		Name:      envVar.Spec.VCenterSimulator,
	}, vCenterSimulator); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get VCenter")
	}
	log = log.WithValues("VCenter", klog.KObj(vCenterSimulator))
	ctx = ctrl.LoggerInto(ctx, log)

	// Fetch the ControlPlaneEndpoint instance
	if envVar.Spec.Cluster.Name == "" {
		return ctrl.Result{}, errors.New("Spec.Cluster.Name cannot be empty")
	}

	controlPlaneEndpoint := &vcsimv1.ControlPlaneEndpoint{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: envVar.Namespace,
		Name:      envVar.Spec.Cluster.Name,
	}, controlPlaneEndpoint); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get ControlPlaneEndpoint")
	}
	log = log.WithValues("ControlPlaneEndpoint", klog.KObj(controlPlaneEndpoint))
	ctx = ctrl.LoggerInto(ctx, log)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(envVar, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the EnvSubst object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, envVar); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted EnvSubst
	if !controlPlaneEndpoint.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, envVar, vCenterSimulator, controlPlaneEndpoint)
	}

	// Handle non-deleted EnvSubst
	return ctrl.Result{}, r.reconcileNormal(ctx, envVar, vCenterSimulator, controlPlaneEndpoint)
}

func (r *EnvVarReconciler) reconcileNormal(ctx context.Context, envVar *vcsimv1.EnvVar, vCenterSimulator *vcsimv1.VCenterSimulator, controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VCSim EnvVar")

	r.lock.Lock()
	defer r.lock.Unlock()

	if r.sshKeys == nil {
		r.sshKeys = map[string]string{}
	}

	key := klog.KObj(vCenterSimulator).String()
	sshKey, ok := r.sshKeys[key]
	if !ok {
		bitSize := 4096

		privateKey, err := generatePrivateKey(bitSize)
		if err != nil {
			return errors.Wrapf(err, "failed to generate private key")
		}

		publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
		if err != nil {
			return errors.Wrapf(err, "failed to generate public key")
		}

		sshKey = string(publicKeyBytes)
		r.sshKeys[key] = sshKey
		log.Info("Created ssh authorized key")
	}

	// Common variables (used both in supervisor and govmomi mode)
	envVar.Status.Variables = map[string]string{
		// cluster template variables about the vcsim instance.
		"VSPHERE_PASSWORD": vCenterSimulator.Status.Password,
		"VSPHERE_USERNAME": vCenterSimulator.Status.Username,

		// Variables for machines ssh key
		"VSPHERE_SSH_AUTHORIZED_KEY": sshKey,

		// other variables required by the cluster template.
		"NAMESPACE":                   vCenterSimulator.Namespace,
		"CLUSTER_NAME":                envVar.Spec.Cluster.Name,
		"KUBERNETES_VERSION":          ptr.Deref(envVar.Spec.Cluster.KubernetesVersion, "v1.28.0"),
		"CONTROL_PLANE_MACHINE_COUNT": strconv.Itoa(int(ptr.Deref(envVar.Spec.Cluster.ControlPlaneMachines, 1))),
		"WORKER_MACHINE_COUNT":        strconv.Itoa(int(ptr.Deref(envVar.Spec.Cluster.WorkerMachines, 1))),

		// variables for the fake APIServer endpoint
		"CONTROL_PLANE_ENDPOINT_IP":   controlPlaneEndpoint.Status.Host,
		"CONTROL_PLANE_ENDPOINT_PORT": strconv.Itoa(int(controlPlaneEndpoint.Status.Port)),

		// variables to set up govc for working with the vcsim instance.
		"GOVC_URL":      fmt.Sprintf("https://%s:%s@%s/sdk", vCenterSimulator.Status.Username, vCenterSimulator.Status.Password, strings.Replace(vCenterSimulator.Status.Host, r.PodIP, "127.0.0.1", 1)), // NOTE: reverting back to local host because the assumption is that the vcsim pod will be port-forwarded on local host
		"GOVC_INSECURE": "true",
	}

	// Variables below are generated using the same utilities used both also for E2E tests setup.
	if r.SupervisorMode {
		config := dependenciesForVCenterSimulator(vCenterSimulator)

		// Variables used only in supervisor mode
		envVar.Status.Variables["VSPHERE_POWER_OFF_MODE"] = ptr.Deref(envVar.Spec.Cluster.PowerOffMode, "trySoft")

		envVar.Status.Variables["VSPHERE_STORAGE_POLICY"] = config.VCenterCluster.StoragePolicy
		envVar.Status.Variables["VSPHERE_IMAGE_NAME"] = config.VCenterCluster.ContentLibrary.Item.Name
		envVar.Status.Variables["VSPHERE_STORAGE_CLASS"] = config.UserNamespace.StorageClass
		envVar.Status.Variables["VSPHERE_MACHINE_CLASS_NAME"] = config.UserNamespace.VirtualMachineClass

		return nil
	}

	// Variables used only in govmomi mode

	// cluster template variables about the vcsim instance.
	envVar.Status.Variables["VSPHERE_SERVER"] = fmt.Sprintf("https://%s", vCenterSimulator.Status.Host)
	envVar.Status.Variables["VSPHERE_TLS_THUMBPRINT"] = vCenterSimulator.Status.Thumbprint
	envVar.Status.Variables["VSPHERE_DATACENTER"] = vcsimhelpers.DatacenterName(int(ptr.Deref(envVar.Spec.Cluster.Datacenter, 0)))
	envVar.Status.Variables["VSPHERE_DATASTORE"] = vcsimhelpers.DatastoreName(int(ptr.Deref(envVar.Spec.Cluster.Datastore, 0)))
	envVar.Status.Variables["VSPHERE_FOLDER"] = fmt.Sprintf("/DC%d/vm", ptr.Deref(envVar.Spec.Cluster.Datacenter, 0))
	envVar.Status.Variables["VSPHERE_NETWORK"] = fmt.Sprintf("/DC%d/network/VM Network", ptr.Deref(envVar.Spec.Cluster.Datacenter, 0))
	envVar.Status.Variables["VSPHERE_RESOURCE_POOL"] = fmt.Sprintf("/DC%d/host/DC%[1]d_C%d/Resources", ptr.Deref(envVar.Spec.Cluster.Datacenter, 0), ptr.Deref(envVar.Spec.Cluster.Cluster, 0))
	envVar.Status.Variables["VSPHERE_STORAGE_POLICY"] = vcsimhelpers.DefaultStoragePolicyName
	envVar.Status.Variables["VSPHERE_TEMPLATE"] = fmt.Sprintf("/DC%d/vm/%s", ptr.Deref(envVar.Spec.Cluster.Datacenter, 0), vcsimhelpers.DefaultVMTemplateName)

	return nil
}

func (r *EnvVarReconciler) reconcileDelete(_ context.Context, _ *vcsimv1.EnvVar, _ *vcsimv1.VCenterSimulator, _ *vcsimv1.ControlPlaneEndpoint) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *EnvVarReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&vcsimv1.EnvVar{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	return nil
}

// generatePrivateKey creates a RSA Private Key of specified byte size.
func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// generatePublicKey take a rsa.PublicKey and return bytes suitable for writing to .pub file
// returns in the format "ssh-rsa ...".
func generatePublicKey(privatekey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

	return pubKeyBytes, nil
}
