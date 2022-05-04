/*
Copyright 2018 The Kubernetes Authors.

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
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"reflect"
	"time"

	"gopkg.in/fsnotify.v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlsig "sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1b1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/controllers"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/version"
)

var (
	setupLog = ctrllog.Log.WithName("entrypoint")

	managerOpts manager.Options
	syncPeriod  time.Duration

	defaultProfilerAddr      = os.Getenv("PROFILER_ADDR")
	defaultSyncPeriod        = manager.DefaultSyncPeriod
	defaultLeaderElectionID  = manager.DefaultLeaderElectionID
	defaultPodName           = manager.DefaultPodName
	defaultWebhookPort       = manager.DefaultWebhookServiceContainerPort
	defaultEnableKeepAlive   = constants.DefaultEnableKeepAlive
	defaultKeepAliveDuration = constants.DefaultKeepAliveDuration
)

func main() {
	rand.Seed(time.Now().UnixNano())

	klog.InitFlags(nil)
	ctrllog.SetLogger(klogr.New())
	if err := flag.Set("v", "2"); err != nil {
		klog.Fatalf("failed to set log level: %v", err)
	}

	flag.StringVar(
		&managerOpts.MetricsBindAddress,
		"metrics-addr",
		"localhost:8080",
		"The address the metric endpoint binds to.")
	flag.BoolVar(
		&managerOpts.LeaderElection,
		"enable-leader-election",
		true,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(
		&managerOpts.LeaderElectionID,
		"leader-election-id",
		defaultLeaderElectionID,
		"Name of the config map to use as the locking resource when configuring leader election.")
	flag.StringVar(
		&managerOpts.Namespace,
		"namespace",
		"",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	profilerAddress := flag.String(
		"profiler-address",
		defaultProfilerAddr,
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")
	flag.DurationVar(
		&syncPeriod,
		"sync-period",
		defaultSyncPeriod,
		"The interval at which cluster-api objects are synchronized")
	flag.IntVar(
		&managerOpts.MaxConcurrentReconciles,
		"max-concurrent-reconciles",
		10,
		"The maximum number of allowed, concurrent reconciles.")
	flag.StringVar(
		&managerOpts.PodName,
		"pod-name",
		defaultPodName,
		"The name of the pod running the controller manager.")
	flag.IntVar(
		&managerOpts.Port,
		"webhook-port",
		defaultWebhookPort,
		"Webhook Server port (set to 0 to disable)")
	flag.StringVar(
		&managerOpts.HealthProbeBindAddress,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)
	flag.StringVar(
		&managerOpts.CredentialsFile,
		"credentials-file",
		"/etc/capv/credentials.yaml",
		"path to CAPV's credentials file",
	)
	flag.BoolVar(
		&managerOpts.EnableKeepAlive,
		"enable-keep-alive",
		defaultEnableKeepAlive,
		"DEPRECATED: feature to enable keep alive handler in vsphere sessions. This functionality is enabled by default now")

	flag.DurationVar(
		&managerOpts.KeepAliveDuration,
		"keep-alive-duration",
		defaultKeepAliveDuration,
		"idle time interval(minutes) in between send() requests in keepalive handler")

	flag.StringVar(
		&managerOpts.NetworkProvider,
		"network-provider",
		"",
		"network provider to be used by Supervisor based clusters.")

	flag.Parse()

	if managerOpts.Namespace != "" {
		setupLog.Info(
			"Watching objects only in namespace for reconciliation",
			"namespace", managerOpts.Namespace)
	}

	if *profilerAddress != "" {
		setupLog.Info(
			"Profiler listening for requests",
			"profiler-address", *profilerAddress)
		go runProfiler(*profilerAddress)
	}
	setupLog.V(1).Info(fmt.Sprintf("feature gates: %+v\n", feature.Gates))

	managerOpts.SyncPeriod = &syncPeriod

	// Create a function that adds all of the controllers and webhooks to the
	// manager.
	addToManager := func(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
		cluster := &v1beta1.VSphereCluster{}
		gvr := v1beta1.GroupVersion.WithResource(reflect.TypeOf(cluster).Elem().Name())
		_, err := mgr.GetRESTMapper().KindFor(gvr)
		if err != nil {
			if meta.IsNoMatchError(err) {
				setupLog.Info(fmt.Sprintf("CRD for %s not loaded, skipping.", gvr.String()))
			} else {
				return err
			}
		} else {
			if err := setupVAPIControllers(ctx, mgr); err != nil {
				return err
			}
		}

		supervisorCluster := &vmwarev1b1.VSphereCluster{}
		gvr = vmwarev1b1.GroupVersion.WithResource(reflect.TypeOf(supervisorCluster).Elem().Name())
		_, err = mgr.GetRESTMapper().KindFor(gvr)
		if err != nil {
			if meta.IsNoMatchError(err) {
				setupLog.Info(fmt.Sprintf("CRD for %s not loaded, skipping.", gvr.String()))
			} else {
				return err
			}
		} else {
			if err := setupSupervisorControllers(ctx, mgr); err != nil {
				return err
			}
		}

		return nil
	}

	setupLog.Info("creating controller manager", "version", version.Get().String())
	managerOpts.AddToManager = addToManager
	mgr, err := manager.New(managerOpts)
	if err != nil {
		setupLog.Error(err, "problem creating controller manager")
		os.Exit(1)
	}

	setupChecks(mgr)

	sigHandler := ctrlsig.SetupSignalHandler()
	setupLog.Info("starting controller manager")
	if err := mgr.Start(sigHandler); err != nil {
		setupLog.Error(err, "problem running controller manager")
		os.Exit(1)
	}

	// initialize notifier for capv-manager-bootstrap-credentials
	watch, err := manager.InitializeWatch(mgr.GetContext(), &managerOpts)
	if err != nil {
		setupLog.Error(err, "failed to initialize watch on CAPV credentials file")
		os.Exit(1)
	}
	defer func(watch *fsnotify.Watcher) {
		_ = watch.Close()
	}(watch)
}

func setupVAPIControllers(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
	if err := (&v1beta1.VSphereClusterTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}

	if err := (&v1beta1.VSphereMachine{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&v1beta1.VSphereMachineList{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}

	if err := (&v1beta1.VSphereMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&v1beta1.VSphereMachineTemplateList{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}

	if err := (&v1beta1.VSphereVM{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&v1beta1.VSphereVMList{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}

	if err := (&v1beta1.VSphereDeploymentZone{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&v1beta1.VSphereDeploymentZoneList{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}

	if err := (&v1beta1.VSphereFailureDomain{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&v1beta1.VSphereFailureDomainList{}).SetupWebhookWithManager(mgr); err != nil {
		return err
	}

	if err := controllers.AddClusterControllerToManager(ctx, mgr, &v1beta1.VSphereCluster{}); err != nil {
		return err
	}
	if err := controllers.AddMachineControllerToManager(ctx, mgr, &v1beta1.VSphereMachine{}); err != nil {
		return err
	}
	if err := controllers.AddVMControllerToManager(ctx, mgr); err != nil {
		return err
	}
	if err := controllers.AddVsphereClusterIdentityControllerToManager(ctx, mgr); err != nil {
		return err
	}
	if err := controllers.AddVSphereDeploymentZoneControllerToManager(ctx, mgr); err != nil {
		return err
	}
	return nil
}

func setupSupervisorControllers(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
	if err := controllers.AddClusterControllerToManager(ctx, mgr, &vmwarev1b1.VSphereCluster{}); err != nil {
		return err
	}

	if err := controllers.AddMachineControllerToManager(ctx, mgr, &vmwarev1b1.VSphereMachine{}); err != nil {
		return err
	}

	if err := controllers.AddServiceAccountProviderControllerToManager(ctx, mgr); err != nil {
		return err
	}

	if err := controllers.AddServiceDiscoveryControllerToManager(ctx, mgr); err != nil {
		return err
	}

	return nil
}

func setupChecks(mgr ctrlmgr.Manager) {
	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func runProfiler(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	_ = http.ListenAndServe(addr, mux)
}
