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
	"crypto/sha1" //nolint: gosec
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"

	_ "github.com/dougm/pretty" // NOTE: this is required to add commands vm.* to cli.Run
	"github.com/pkg/errors"
	_ "github.com/vmware/govmomi/govc/vm" // NOTE: this is required to add commands vm.* to cli.Run
	pbmsimulator "github.com/vmware/govmomi/pbm/simulator"
	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator" // NOTE: this is required to content library & other vapi methods to the simulator
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/framework/vmoperator"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/controllers/images"
)

const (
	// vmIP reconciler secret.

	netConfigMapName       = "vsphere.provider.config.netoperator.vmware.com"
	netConfigServerURLKey  = "server"
	netConfigDatacenterKey = "datacenter"
	netConfigUsernameKey   = "username"
	netConfigPasswordKey   = "password"
	netConfigThumbprintKey = "thumbprint"
)

type VCenterSimulatorReconciler struct {
	Client         client.Client
	SupervisorMode bool

	vcsimInstances map[string]*vcsim.Simulator
	lock           sync.RWMutex

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=vcentersimulators,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=vcsim.infrastructure.cluster.x-k8s.io,resources=vcentersimulators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=topology.tanzu.vmware.com,resources=availabilityzones,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachineclasses,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachineclassbindings,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=contentlibraryproviders,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=contentsources,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=contentsourcebindings,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachineimages,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create

func (r *VCenterSimulatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Fetch the VCenterSimulator instance
	vCenterSimulator := &vcsimv1.VCenterSimulator{}
	if err := r.Client.Get(ctx, req.NamespacedName, vCenterSimulator); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(vCenterSimulator, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the VCenter object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, vCenterSimulator); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted machines
	if !vCenterSimulator.DeletionTimestamp.IsZero() {
		r.reconcileDelete(ctx, vCenterSimulator)
		return ctrl.Result{}, nil
	}

	// Add finalizer first if not set to avoid the race condition between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp is not set.
	if !controllerutil.ContainsFinalizer(vCenterSimulator, vcsimv1.VCenterFinalizer) {
		controllerutil.AddFinalizer(vCenterSimulator, vcsimv1.VCenterFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle non-deleted machines
	return ctrl.Result{}, r.reconcileNormal(ctx, vCenterSimulator)
}

func (r *VCenterSimulatorReconciler) reconcileNormal(ctx context.Context, vCenterSimulator *vcsimv1.VCenterSimulator) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling VCenter")

	r.lock.Lock()
	defer r.lock.Unlock()

	if r.vcsimInstances == nil {
		r.vcsimInstances = map[string]*vcsim.Simulator{}
	}

	key := klog.KObj(vCenterSimulator).String()
	if _, ok := r.vcsimInstances[key]; !ok {
		// Define the model for the VCSim instance, starting from simulator.VPX
		// and changing version + all the setting specified in the spec.
		// NOTE: it is necessary to create the model before passing it to the builder
		// in order to register the endpoint for handling request about storage policies.
		model := simulator.VPX()
		model.ServiceContent.About.Version = vcsimMinVersionForCAPV
		if vCenterSimulator.Spec.Model != nil {
			model.ServiceContent.About.Version = ptr.Deref(vCenterSimulator.Spec.Model.VSphereVersion, model.ServiceContent.About.Version)
			model.Datacenter = int(ptr.Deref(vCenterSimulator.Spec.Model.Datacenter, int32(model.Datacenter)))
			model.Cluster = int(ptr.Deref(vCenterSimulator.Spec.Model.Cluster, int32(model.Cluster)))
			model.ClusterHost = int(ptr.Deref(vCenterSimulator.Spec.Model.ClusterHost, int32(model.ClusterHost)))
			model.Pool = int(ptr.Deref(vCenterSimulator.Spec.Model.Pool, int32(model.Pool)))
			model.Datastore = int(ptr.Deref(vCenterSimulator.Spec.Model.Datastore, int32(model.Datastore)))
		}
		if err := model.Create(); err != nil {
			return errors.Wrapf(err, "failed to create vcsim server model")
		}
		model.Service.RegisterSDK(pbmsimulator.New())

		// Compute the vcsim URL, binding all interfaces (so it will be accessible both from other pods via the pod IP and via kubectl port-forward on local host);
		// a random port will be used unless we are reconciling a previously existing vCenterSimulator after a restart;
		// in case of restart it will try to re-use the port previously assigned, but the internal status of vcsim will be lost.
		// NOTE: re-using the same port might be racy with other vcsimURL being created using a random port,
		// but we consider this risk acceptable for testing purposes.
		host := "0.0.0.0"
		port := "0"
		if vCenterSimulator.Status.Host != "" {
			_, port, _ = net.SplitHostPort(vCenterSimulator.Status.Host)
		}
		vcsimURL, err := url.Parse(fmt.Sprintf("https://%s", net.JoinHostPort(host, port)))
		if err != nil {
			return errors.Wrapf(err, "failed to parse vcsim server url")
		}

		// Start the vcsim instance
		vcsimInstance, err := vcsim.NewBuilder().
			WithModel(model).
			SkipModelCreate().
			WithURL(vcsimURL).
			Build()

		if err != nil {
			return errors.Wrapf(err, "failed to create vcsim server instance")
		}
		r.vcsimInstances[key] = vcsimInstance
		log.Info("Created vcsim server", "url", vcsimInstance.ServerURL())

		vCenterSimulator.Status.Host = vcsimInstance.ServerURL().Host
		vCenterSimulator.Status.Username = vcsimInstance.Username()
		vCenterSimulator.Status.Password = vcsimInstance.Password()

		// Add a VM template
		// Note: for the sake of testing with vcsim the template doesn't really matter (nor the version of K8s hosted on it)
		// so we create only a VM template with a well-known name.
		if err := createVMTemplate(ctx, vCenterSimulator); err != nil {
			return err
		}
	}

	if vCenterSimulator.Status.Thumbprint == "" {
		// Compute the Thumbprint out of the certificate self-generated by vcsim.
		config := &tls.Config{InsecureSkipVerify: true} //nolint: gosec
		addr := vCenterSimulator.Status.Host
		conn, err := tls.Dial("tcp", addr, config)
		if err != nil {
			return errors.Wrapf(err, "failed to connect to vcsim server instance to infer thumbprint")
		}
		defer conn.Close()

		cert := conn.ConnectionState().PeerCertificates[0]
		vCenterSimulator.Status.Thumbprint = ThumbprintSHA1(cert)
	}

	if r.SupervisorMode {
		// In order to run the vm-operator in standalone mode it is required to provide it with the dependencies it needs to work:
		// - A set of objects/configurations in the vCenterSimulator cluster the vm-operator is pointing to
		// - A set of Kubernetes object the vm-operator relies on

		// To mimic the supervisor cluster, there will be only one vm-operator instance for each management cluster;
		// also, the logic below should consider that the instance of the vm-operator is bound to a specific vCenterSimulator cluster.

		// Those are config for vCenterSimulator cluster DC0/C0, datastore LocalDS_0 in vcsim.
		datacenter := 0
		cluster := 0
		datastore := 0

		config := vmoperator.Dependencies{
			// This is where tilt deploys the vm-operator
			Namespace: vmoperator.DefaultNamespace,

			VCenterCluster: vmoperator.VCenterClusterConfig{
				ServerURL:       vCenterSimulator.Status.Host,
				Username:        vCenterSimulator.Status.Username,
				Password:        vCenterSimulator.Status.Password,
				Thumbprint:      vCenterSimulator.Status.Thumbprint,
				Datacenter:      vcsimDatacenterName(datacenter),
				Cluster:         vcsimClusterPath(datacenter, cluster),
				Folder:          vcsimVMFolderName(datacenter),
				ResourcePool:    vcsimResourcePoolPath(datacenter, cluster),
				StoragePolicyID: vcsimDefaultStoragePolicyName,

				// Those are settings for a fake content library we are going to create given that it doesn't exists in vcsim by default.
				ContentLibrary: vmoperator.ContentLibraryConfig{
					Name:      "kubernetes",
					Datastore: vcsimDatastorePath(datacenter, datastore),
					Item: vmoperator.ContentLibraryItemConfig{
						Name: "test-image-ovf",
						Files: []vmoperator.ContentLibraryItemFilesConfig{ // TODO: check if we really need both
							{
								Name:    "ttylinux-pc_i486-16.1.ovf",
								Content: images.SampleOVF,
							},
						},
						ItemType:    "ovf",
						ProductInfo: "dummy-productInfo",
						OSInfo:      "dummy-OSInfo",
					},
				},
			},

			// The users are expected to store Cluster API clusters to be managed by the vm-operator
			// in the default namespace and to use the "vcsim-default" storage class.
			UserNamespace: vmoperator.UserNamespaceConfig{
				Name:         corev1.NamespaceDefault,
				StorageClass: "vcsim-default",
			},
		}

		if err := vmoperator.ReconcileDependencies(ctx, r.Client, config); err != nil {
			return err
		}

		// The vm-operator doesn't take care of the networking part of the VM, which is usually
		// managed by other components in the supervisor cluster.
		// In order to make things to work in vcsim, there is the vmIP reconciler, which requires
		// some info about the vcsim instance; in order to do so, we are creating a Secret.

		if err := addPreRequisitesForVMIPreconciler(ctx, r.Client, config); err != nil {
			return err
		}
	}

	return nil
}

func addPreRequisitesForVMIPreconciler(ctx context.Context, c client.Client, config vmoperator.Dependencies) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling requirements for the Fake net-operator Deployment")

	netOperatorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      netConfigMapName,
			Namespace: config.Namespace,
		},
		StringData: map[string]string{
			netConfigServerURLKey:  config.VCenterCluster.ServerURL,
			netConfigDatacenterKey: config.VCenterCluster.Datacenter,
			netConfigUsernameKey:   config.VCenterCluster.Username,
			netConfigPasswordKey:   config.VCenterCluster.Password,
			netConfigThumbprintKey: config.VCenterCluster.Thumbprint,
		},
		Type: corev1.SecretTypeOpaque,
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(netOperatorSecret), netOperatorSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get net-operator Secret %s", netOperatorSecret.Name)
		}
		if err := c.Create(ctx, netOperatorSecret); err != nil {
			return errors.Wrapf(err, "failed to create net-operator Secret %s", netOperatorSecret.Name)
		}
		log.Info("Created net-operator Secret", "Secret", klog.KObj(netOperatorSecret))
	}

	return nil
}

func (r *VCenterSimulatorReconciler) reconcileDelete(ctx context.Context, vCenterSimulator *vcsimv1.VCenterSimulator) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling delete VCenter server")

	r.lock.Lock()
	defer r.lock.Unlock()

	key := klog.KObj(vCenterSimulator).String()
	vcsimInstance, ok := r.vcsimInstances[key]
	if ok {
		log.Info("Deleting vcsim server")
		vcsimInstance.Destroy()
		delete(r.vcsimInstances, key)
	}

	controllerutil.RemoveFinalizer(vCenterSimulator, vcsimv1.VCenterFinalizer)
}

// SetupWithManager will add watches for this controller.
func (r *VCenterSimulatorReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&vcsimv1.VCenterSimulator{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	return nil
}

// ThumbprintSHA1 returns the thumbprint of the given cert in the same format used by the SDK and Client.SetThumbprint.
func ThumbprintSHA1(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw) //nolint: gosec
	hex := make([]string, len(sum))
	for i, b := range sum {
		hex[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(hex, ":")
}
