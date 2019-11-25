/*
Copyright 2019 The Kubernetes Authors.

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

package manager

import (
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	// +kubebuilder:scaffold:imports

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// AddToManagerFunc is a function that can be optionally specified with
// the manager's Options in order to explicitly decide what controllers and
// webhooks to add to the manager.
type AddToManagerFunc func(*context.ControllerManagerContext, ctrlmgr.Manager) error

// Options describes the options used to create a new CAPV manager.
type Options struct {
	// LeaderElectionEnabled is a flag that enables leader election.
	LeaderElectionEnabled bool

	// LeaderElectionID is the name of the config map to use as the
	// locking resource when configuring leader election.
	LeaderElectionID string

	// SyncPeriod is the amount of time to wait between syncing the local
	// object cache with the API server.
	SyncPeriod time.Duration

	// MaxConcurrentReconciles the maximum number of allowed, concurrent
	// reconciles.
	//
	// Defaults to the eponymous constant in this package.
	MaxConcurrentReconciles int

	// MetricsAddr is the net.Addr string for the metrics server.
	MetricsAddr string

	// PodNamespace is the namespace in which the pod running the controller
	// manager is located.
	//
	// Defaults to the eponymous constant in this package.
	PodNamespace string

	// PodName is the name of the pod running the controller manager.
	//
	// Defaults to the eponymous constant in this package.
	PodName string

	// WatchNamespace is the namespace the controllers watch for changes. If
	// no value is specified then all namespaces are watched.
	//
	// Defaults to the eponymous constant in this package.
	WatchNamespace string

	// Username is the username for the account used to access remote vSphere
	// endpoints.
	Username string

	// Password is the password for the account used to access remote vSphere
	// endpoints.
	Password string

	Logger     logr.Logger
	KubeConfig *rest.Config
	Scheme     *runtime.Scheme
	NewCache   cache.NewCacheFunc

	// AddToManager is a function that can be optionally specified with
	// the manager's Options in order to explicitly decide what controllers
	// and webhooks to add to the manager.
	AddToManager AddToManagerFunc
}

func (o *Options) defaults() {
	if o.Logger == nil {
		o.Logger = ctrllog.Log
	}

	if o.PodNamespace == "" {
		o.PodNamespace = DefaultPodNamespace
	}

	if o.PodName == "" {
		o.PodName = DefaultPodName
	}

	if o.SyncPeriod == 0 {
		o.SyncPeriod = DefaultSyncPeriod
	}

	if o.KubeConfig == nil {
		o.KubeConfig = config.GetConfigOrDie()
	}

	if o.Scheme == nil {
		o.Scheme = runtime.NewScheme()
	}

	if o.WatchNamespace == "" {
		o.WatchNamespace = DefaultWatchNamespace
	}

	if o.MaxConcurrentReconciles == 0 {
		o.MaxConcurrentReconciles = DefaultMaxConcurrentReconciles
	}

	if o.Username == "" {
		o.Username = os.Getenv("VSPHERE_USERNAME")
	}

	if o.Password == "" {
		o.Password = os.Getenv("VSPHERE_PASSWORD")
	}
}
