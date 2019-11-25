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
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"time"

	"k8s.io/klog"
	"k8s.io/klog/klogr"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	ctrlsig "sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"sigs.k8s.io/cluster-api-provider-vsphere/controllers"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

var (
	managerOpts                    manager.Options
	defaultProfilerAddr            = os.Getenv("PROFILER_ADDR")
	defaultSyncPeriod              = manager.DefaultSyncPeriod
	defaultMaxConcurrentReconciles = manager.DefaultMaxConcurrentReconciles
	defaultLeaderElectionID        = manager.DefaultLeaderElectionID
	defaultPodNamespace            = manager.DefaultPodNamespace
	defaultPodName                 = manager.DefaultPodName
	defaultWatchNamespace          = manager.DefaultWatchNamespace
)

func init() {
	if v, err := time.ParseDuration(os.Getenv("SYNC_PERIOD")); err == nil {
		defaultSyncPeriod = v
	}
	if v, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_RECONCILES")); err == nil {
		defaultMaxConcurrentReconciles = v
	}
	if v := os.Getenv("LEADER_ELECTION_ID"); v != "" {
		defaultLeaderElectionID = v
	}
	if v := os.Getenv("POD_NAMESPACE"); v != "" {
		defaultPodNamespace = v
	}
	if v := os.Getenv("POD_NAME"); v != "" {
		defaultPodName = v
	}
	if v := os.Getenv("WATCH_NAMESPACE"); v != "" {
		defaultWatchNamespace = v
	}
}

func main() {
	klog.InitFlags(nil)
	ctrllog.SetLogger(klogr.New())
	setupLog := ctrllog.Log.WithName("entrypoint")
	if err := flag.Set("v", "2"); err != nil {
		klog.Fatalf("failed to set log level: %v", err)
	}

	flag.StringVar(
		&managerOpts.MetricsAddr,
		"metrics-addr",
		":8084",
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
		"watch-namespace",
		defaultWatchNamespace,
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.")
	profilerAddress := flag.String(
		"profiler-address",
		defaultProfilerAddr,
		"Bind address to expose the pprof profiler")
	flag.DurationVar(
		&managerOpts.SyncPeriod,
		"sync-period",
		defaultSyncPeriod,
		"The interval at which cluster-api objects are synchronized")
	flag.IntVar(
		&managerOpts.MaxConcurrentReconciles,
		"max-concurrent-reconciles",
		defaultMaxConcurrentReconciles,
		"The maximum number of allowed, concurrent reconciles.")
	flag.StringVar(
		&managerOpts.PodNamespace,
		"pod-namespace",
		defaultPodNamespace,
		"The namespace in which the pod running the controller manager is located.")
	flag.StringVar(
		&managerOpts.PodName,
		"pod-name",
		defaultPodName,
		"The name of the pod running the controller manager.")

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
		if err := controllers.AddClusterControllerToManager(ctx, mgr); err != nil {
			return err
		}
		if err := controllers.AddMachineControllerToManager(ctx, mgr); err != nil {
			return err
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

	sigHandler := ctrlsig.SetupSignalHandler()
	setupLog.Info("starting controller manager")
	if err := mgr.Start(sigHandler); err != nil {
		setupLog.Error(err, "problem running controller manager")
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
