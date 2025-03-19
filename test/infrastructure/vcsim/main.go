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

// Package main define main for the vcsim controller.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"reflect"
	goruntime "runtime"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/feature"
	inmemoryruntime "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime"
	inmemoryserver "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/apiwarnings"
	"sigs.k8s.io/cluster-api/util/flags"
	"sigs.k8s.io/cluster-api/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	topologyv1 "sigs.k8s.io/cluster-api-provider-vsphere/internal/apis/topology/v1alpha1"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/controllers"
)

var (
	inmemoryScheme = runtime.NewScheme()
	scheme         = runtime.NewScheme()
	setupLog       = ctrl.Log.WithName("setup")
	controllerName = "cluster-api-vcsim-controller-manager"

	// common flags flags.
	enableLeaderElection        bool
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	watchFilterValue            string
	watchNamespace              string
	profilerAddress             string
	enableContentionProfiling   bool
	syncPeriod                  time.Duration
	restConfigQPS               float32
	restConfigBurst             int
	healthAddr                  string
	managerOptions              = flags.ManagerOptions{}
	logOptions                  = logs.NewOptions()
	// vcsim specific flags.
	vSphereVMConcurrency              int
	virtualMachineConcurrency         int
	vCenterSimulatorConcurrency       int
	controlPlaneEndpointConcurrency   int
	envsubstConcurrency               int
	vmOperatorDependenciesConcurrency int
)

func init() {
	// scheme used for operating on the management cluster.
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = vcsimv1.AddToScheme(scheme)
	_ = topologyv1.AddToScheme(scheme)
	_ = vmoprv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = vmwarev1.AddToScheme(scheme)

	// scheme used for operating in memory.
	_ = corev1.AddToScheme(inmemoryScheme)
	_ = appsv1.AddToScheme(inmemoryScheme)
	_ = rbacv1.AddToScheme(inmemoryScheme)
	_ = infrav1.AddToScheme(inmemoryScheme)
}

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(logOptions, fs)

	fs.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	fs.DurationVar(&leaderElectionLeaseDuration, "leader-elect-lease-duration", 15*time.Second,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)")

	fs.DurationVar(&leaderElectionRenewDeadline, "leader-elect-renew-deadline", 10*time.Second,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)")

	fs.DurationVar(&leaderElectionRetryPeriod, "leader-elect-retry-period", 2*time.Second,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)")

	fs.StringVar(&watchNamespace, "namespace", "",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")

	fs.StringVar(&watchFilterValue, "watch-filter", "",
		fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel))

	fs.StringVar(&profilerAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	fs.BoolVar(&enableContentionProfiling, "contention-profiling", false,
		"Enable block profiling")

	fs.IntVar(&vSphereVMConcurrency, "vsphere-vm-concurrency", 10,
		"Number of VSphereVM to process simultaneously")

	fs.IntVar(&virtualMachineConcurrency, "virtual-machine-concurrency", 10,
		"Number of VirtualMachine to process simultaneously")

	fs.IntVar(&vCenterSimulatorConcurrency, "vcenter-simulator-concurrency", 10,
		"Number of VCenterSimulator to process simultaneously")

	fs.IntVar(&controlPlaneEndpointConcurrency, "controlplane-endpoint-concurrency", 10,
		"Number of ControlPlaneEndpoint to process simultaneously")

	fs.IntVar(&envsubstConcurrency, "envsubst-concurrency", 10,
		"Number of Envsubst to process simultaneously")

	fs.IntVar(&vmOperatorDependenciesConcurrency, "vm-operator-dependencies-concurrency", 10,
		"Number of VMOperatorDependencies to process simultaneously")

	fs.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	fs.Float32Var(&restConfigQPS, "kube-api-qps", 100,
		"Maximum queries per second from the controller client to the Kubernetes API server.")

	fs.IntVar(&restConfigBurst, "kube-api-burst", 200,
		"Maximum number of queries that should be allowed in one burst from the controller client to the Kubernetes API server.")

	fs.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")

	flags.AddManagerOptions(fs, &managerOptions)

	feature.MutableGates.AddFlag(fs)
}

// Add RBAC for the authorized diagnostics endpoint.
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func main() {
	setupLog.Info(fmt.Sprintf("Version: %+v", version.Get().String()))

	InitFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	// Set log level 2 as default.
	if err := pflag.CommandLine.Set("v", "2"); err != nil {
		setupLog.Error(err, "failed to set default log level")
		os.Exit(1)
	}
	pflag.Parse()

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// klog.Background will automatically use the right logger.
	ctrl.SetLogger(klog.Background())

	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = restConfigQPS
	restConfig.Burst = restConfigBurst
	restConfig.UserAgent = remote.DefaultClusterAPIUserAgent(controllerName)
	restConfig.WarningHandler = apiwarnings.DefaultHandler(klog.Background().WithName("API Server Warning"))

	_, metricsOptions, err := flags.GetManagerOptions(managerOptions)
	if err != nil {
		setupLog.Error(err, "Unable to start manager: invalid flags")
		os.Exit(1)
	}
	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	if enableContentionProfiling {
		goruntime.SetBlockProfileRate(1)
	}

	ctrlOptions := ctrl.Options{
		Scheme:                     scheme,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "vcsim-controller-leader-election-capi",
		LeaseDuration:              &leaderElectionLeaseDuration,
		RenewDeadline:              &leaderElectionRenewDeadline,
		RetryPeriod:                &leaderElectionRetryPeriod,
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		HealthProbeBindAddress:     healthAddr,
		PprofBindAddress:           profilerAddress,
		Metrics:                    *metricsOptions,
		Cache: cache.Options{
			DefaultNamespaces: watchNamespaces,
			SyncPeriod:        &syncPeriod,
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
			},
		},
		// WebhookServer: webhook.NewServer(
		//	webhook.Options{
		//		Port:    webhookPort,
		//		CertDir: webhookCertDir,
		//		TLSOpts: tlsOptionOverrides,
		//	},
		// ),
	}

	mgr, err := ctrl.NewManager(restConfig, ctrlOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	// Check for non-supervisor VSphereCluster and start controller if found
	gvr := infrav1.GroupVersion.WithResource(reflect.TypeOf(&infrav1.VSphereCluster{}).Elem().Name())
	govmomiMode, err := isCRDDeployed(mgr, gvr)
	if err != nil {
		setupLog.Error(err, "unable to detect govmomi mode")
		os.Exit(1)
	}

	// Check for supervisor VSphereCluster and start controller if found
	gvr = vmwarev1.GroupVersion.WithResource(reflect.TypeOf(&vmwarev1.VSphereCluster{}).Elem().Name())
	supervisorMode, err := isCRDDeployed(mgr, gvr)
	if err != nil {
		setupLog.Error(err, "unable to detect supervisor mode")
		os.Exit(1)
	}

	// Continuing startup does not make sense without having managers added.
	if !govmomiMode && !supervisorMode {
		err := errors.New("neither supervisor nor govmomi CRDs detected")
		setupLog.Error(err, "CAPV CRDs are not deployed yet, restarting")
		os.Exit(1)
	}

	setupChecks(mgr, supervisorMode)
	setupIndexes(ctx, mgr, supervisorMode)
	setupReconcilers(ctx, mgr, supervisorMode)
	setupWebhooks(mgr, supervisorMode)

	setupLog.Info("starting manager", "version", version.Get().String())
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupChecks(mgr ctrl.Manager, _ bool) {
	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func setupIndexes(_ context.Context, _ ctrl.Manager, _ bool) {
}

func setupReconcilers(ctx context.Context, mgr ctrl.Manager, supervisorMode bool) {
	// Start cloud manager
	inmemoryManager := inmemoryruntime.NewManager(inmemoryScheme)
	if err := inmemoryManager.Start(ctx); err != nil {
		setupLog.Error(err, "unable to start a cloud manager")
		os.Exit(1)
	}

	// Start an http server
	podIP := os.Getenv("POD_IP")
	apiServerMux, err := inmemoryserver.NewWorkloadClustersMux(inmemoryManager, podIP)
	if err != nil {
		setupLog.Error(err, "unable to create workload clusters mux")
		os.Exit(1)
	}

	// Setup reconcilers
	if err := (&controllers.VCenterSimulatorReconciler{
		Client:           mgr.GetClient(),
		SupervisorMode:   supervisorMode,
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(vCenterSimulatorConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VCenterSimulatorReconciler")
		os.Exit(1)
	}

	if err := (&controllers.ControlPlaneEndpointReconciler{
		Client:           mgr.GetClient(),
		InMemoryManager:  inmemoryManager,
		APIServerMux:     apiServerMux,
		PodIP:            podIP,
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(controlPlaneEndpointConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ControlPlaneEndpointReconciler")
		os.Exit(1)
	}

	if supervisorMode {
		if err := (&controllers.VirtualMachineReconciler{
			Client:           mgr.GetClient(),
			InMemoryManager:  inmemoryManager,
			APIServerMux:     apiServerMux,
			WatchFilterValue: watchFilterValue,
		}).SetupWithManager(ctx, mgr, concurrency(virtualMachineConcurrency)); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "VirtualMachineReconciler")
			os.Exit(1)
		}

		if err := (&controllers.VMOperatorDependenciesReconciler{
			Client:           mgr.GetClient(),
			WatchFilterValue: watchFilterValue,
		}).SetupWithManager(ctx, mgr, concurrency(vmOperatorDependenciesConcurrency)); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "VMOperatorDependenciesReconciler")
			os.Exit(1)
		}
	} else {
		if err := (&controllers.VSphereVMReconciler{
			Client:           mgr.GetClient(),
			InMemoryManager:  inmemoryManager,
			APIServerMux:     apiServerMux,
			WatchFilterValue: watchFilterValue,
		}).SetupWithManager(ctx, mgr, concurrency(vSphereVMConcurrency)); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "VSphereVMReconciler")
			os.Exit(1)
		}
	}

	if err := (&controllers.EnvVarReconciler{
		Client:           mgr.GetClient(),
		SupervisorMode:   supervisorMode,
		PodIP:            podIP,
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(envsubstConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "EnvVarReconciler")
		os.Exit(1)
	}
}

func setupWebhooks(_ ctrl.Manager, _ bool) {
}

func concurrency(c int) controller.Options {
	return controller.Options{MaxConcurrentReconciles: c}
}

func isCRDDeployed(mgr ctrlmgr.Manager, gvr schema.GroupVersionResource) (bool, error) {
	_, err := mgr.GetRESTMapper().KindFor(gvr)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
