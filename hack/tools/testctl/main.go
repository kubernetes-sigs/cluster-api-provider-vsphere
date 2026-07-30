/*
Copyright The Kubernetes Authors.

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

// Package main implements the testctl CLI.
package main

import (
	"fmt"
	"os"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/core"
	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/testctl/plugins"
)

var (
	logOptions = logs.NewOptions()
	scheme     = runtime.NewScheme()
	rc         = runOptions{}
)

type stackTracer interface {
	StackTrace() pkgerrors.StackTrace
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		if err, ok := err.(stackTracer); ok {
			for _, f := range err.StackTrace() {
				fmt.Fprintf(os.Stderr, "%+s:%d\n", f, f)
			}
		}
		os.Exit(1)
	}
}

// RootCmd is testctl's root CLI command.
var RootCmd = &cobra.Command{
	Use:          "testctl",
	SilenceUsage: true,
	Short:        "testctl runs Cluster API tests",
	RunE:         runTest,
}

type runOptions struct {
	kubeconfig        string
	kubeconfigContext string
	config            string
}

func init() {
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1.AddToScheme(scheme))

	logsv1.AddFlags(logOptions, RootCmd.Flags())

	RootCmd.Flags().StringVar(&rc.kubeconfig, "kubeconfig", "",
		"Path to a kubeconfig file to use for the management cluster. If empty, default discovery rules apply.")
	RootCmd.Flags().StringVar(&rc.kubeconfigContext, "kubeconfig-context", "",
		"Context to be used within the kubeconfig file. If empty, current context will be used.")
	RootCmd.Flags().StringVarP(&rc.config, "config", "c", "",
		"The config file with the test sequence to be run.")
}

func runTest(_ *cobra.Command, _ []string) error {
	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		return err
	}

	ctrl.SetLogger(klog.Background())
	ctx := ctrl.SetupSignalHandler()

	configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if rc.kubeconfig != "" {
		configLoadingRules.ExplicitPath = rc.kubeconfig
	}

	config, err := configLoadingRules.Load()
	if err != nil {
		return pkgerrors.Wrap(err, "failed to load Kubeconfig")
	}

	contextName := config.CurrentContext
	if rc.kubeconfigContext != "" {
		contextName = rc.kubeconfigContext
	}

	context, ok := config.Contexts[contextName]
	if !ok {
		if rc.kubeconfig != "" {
			return pkgerrors.Errorf("failed to get context %q from %q", contextName, configLoadingRules.GetExplicitFile())
		}
		return pkgerrors.Errorf("failed to get context %q from %q", contextName, configLoadingRules.GetLoadingPrecedence())
	}

	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{
		Context: *context,
	}).ClientConfig()
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to create rest config from %q", configLoadingRules.GetExplicitFile())
	}
	restConfig.UserAgent = "testctl"
	restConfig.QPS = 20
	restConfig.Burst = 100

	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to create client from %q", configLoadingRules.GetExplicitFile())
	}

	if _, err := os.Stat(rc.config); pkgerrors.Is(err, os.ErrNotExist) {
		return pkgerrors.Errorf("file %s does not exist", rc.config)
	}

	configData, err := os.ReadFile(rc.config)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to read file %s", rc.config)
	}

	return core.Run(ctx, c, configData, plugins.Catalog())
}
