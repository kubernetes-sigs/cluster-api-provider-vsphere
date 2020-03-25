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
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"

	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlsig "sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"sigs.k8s.io/cluster-api-provider-vsphere/controllers"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

var (
	setupLog = ctrllog.Log.WithName("entrypoint")

	managerOpts             manager.Options
	defaultProfilerAddr     = os.Getenv("PROFILER_ADDR")
	defaultSyncPeriod       = manager.DefaultSyncPeriod
	defaultLeaderElectionID = manager.DefaultLeaderElectionID
	defaultPodName          = manager.DefaultPodName
	defaultWebhookPort      = manager.DefaultWebhookServiceContainerPort
)

// nolint:gocognit
func main() {
	rand.Seed(time.Now().UnixNano())

	klog.InitFlags(nil)
	ctrllog.SetLogger(klogr.New())
	if err := flag.Set("v", "2"); err != nil {
		klog.Fatalf("failed to set log level: %v", err)
	}

	flag.StringVar(
		&managerOpts.MetricsAddr,
		"metrics-addr",
		":8080",
		"The address the metric endpoint binds to.")
	flag.BoolVar(
		&managerOpts.LeaderElectionEnabled,
		"enable-leader-election",
		true,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(
		&managerOpts.LeaderElectionID,
		"leader-election-id",
		defaultLeaderElectionID,
		"Name of the config map to use as the locking resource when configuring leader election.")
	flag.StringVar(
		&managerOpts.WatchNamespace,
		"namespace",
		"",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	profilerAddress := flag.String(
		"profiler-address",
		defaultProfilerAddr,
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")
	flag.DurationVar(
		&managerOpts.SyncPeriod,
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
		&managerOpts.WebhookPort,
		"webhook-port",
		defaultWebhookPort,
		"Webhook Server port (set to 0 to disable)")
	flag.StringVar(
		&managerOpts.HealthAddr,
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

	flag.Parse()

	if managerOpts.WatchNamespace != "" {
		setupLog.Info(
			"Watching objects only in namespace for reconciliation",
			"namespace", managerOpts.WatchNamespace)
	}

	if *profilerAddress != "" {
		setupLog.Info(
			"Profiler listening for requests",
			"profiler-address", *profilerAddress)
		go runProfiler(*profilerAddress)
	}

	// Create a function that adds all of the controllers and webhooks to the
	// manager.
	addToManager := func(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
		if managerOpts.WebhookPort != 0 {
			if err := (&v1alpha3.VSphereCluster{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha3.VSphereClusterList{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}

			if err := (&v1alpha3.VSphereMachine{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha3.VSphereMachineList{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}

			if err := (&v1alpha3.VSphereMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha3.VSphereMachineTemplateList{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}

			if err := (&v1alpha3.VSphereVM{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha3.VSphereVMList{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}

			if err := (&v1alpha3.HAProxyLoadBalancer{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha3.HAProxyLoadBalancerList{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha2.VSphereCluster{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha2.VSphereMachine{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
			if err := (&v1alpha2.VSphereMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
				return err
			}
		} else {
			if err := controllers.AddClusterControllerToManager(ctx, mgr); err != nil {
				return err
			}
			if err := controllers.AddMachineControllerToManager(ctx, mgr); err != nil {
				return err
			}
			if err := controllers.AddVMControllerToManager(ctx, mgr); err != nil {
				return err
			}
			if err := controllers.AddHAProxyLoadBalancerControllerToManager(ctx, mgr); err != nil {
				return err
			}
		}

		return nil
	}

	setupLog.Info("creating controller manager")
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
}

func setupChecks(mgr ctrlmgr.Manager) {
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
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
