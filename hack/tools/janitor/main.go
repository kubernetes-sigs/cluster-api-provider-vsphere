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

// Package main is the main package for capv-janitor.
package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ipamScheme *runtime.Scheme

func init() {
	ipamScheme = runtime.NewScheme()
	_ = ipamv1.AddToScheme(ipamScheme)
}

var (
	dryRun         bool
	ipamNamespace  string
	maxAge         time.Duration
	vsphereFolders []string
)

func initFlags(fs *pflag.FlagSet) {
	fs.StringArrayVar(&vsphereFolders, "folder", []string{}, "Path to folders in vCenter to cleanup virtual machines.")
	fs.StringVar(&ipamNamespace, "ipam-namespace", "", "Namespace for IPAddressClaim cleanup.")
	fs.DurationVar(&maxAge, "max-age", time.Hour*12, "Maximum age of an object before it is getting deleted.")
	fs.BoolVar(&dryRun, "dry-run", false, "dry-run results in not deleting anything but printing the actions.")
}

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	log := klog.Background()
	ctx := ctrl.LoggerInto(context.Background(), log)

	if err := run(ctx); err != nil {
		log.Error(err, "Failed running vsphere-janitor")
		os.Exit(1)
	}

	log.Info("Finished cleanup.")
}

func run(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Configured settings", "dry-run", dryRun)
	log.Info("Configured settings", "folders", vsphereFolders)
	log.Info("Configured settings", "ipam-namespace", ipamNamespace)
	log.Info("Configured settings", "max-age", maxAge)

	// Create clients for vSphere.
	vSphereClients, err := newVSphereClients(ctx, getVSphereClientInput{
		Username:   os.Getenv("GOVC_USERNAME"),
		Password:   os.Getenv("GOVC_PASSWORD"),
		Server:     os.Getenv("GOVC_URL"),
		Thumbprint: os.Getenv("VSPHERE_TLS_THUMBPRINT"),
		UserAgent:  "capv-janitor",
	})
	if err != nil {
		return errors.Wrap(err, "creating vSphere clients")
	}
	defer vSphereClients.logout(ctx)

	// Create controller-runtime client for IPAM.
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "unable to get kubeconfig")
	}
	ipamClient, err := client.New(restConfig, client.Options{Scheme: ipamScheme})
	if err != nil {
		return errors.Wrap(err, "creating IPAM client")
	}

	janitor := newJanitor(vSphereClients, ipamClient, maxAge, ipamNamespace, dryRun)

	// First cleanup old vms to free up IPAddressClaims or cluster modules which are still in-use.
	errList := []error{}
	for _, folder := range vsphereFolders {
		if err := janitor.deleteVSphereVMs(ctx, folder); err != nil {
			errList = append(errList, errors.Wrapf(err, "cleaning up vSphereVMs for folder %q", folder))
		}
	}
	if err := kerrors.NewAggregate(errList); err != nil {
		return errors.Wrap(err, "cleaning up vSphereVMs")
	}

	// Second cleanup IPAddressClaims.
	if err := janitor.deleteIPAddressClaims(ctx); err != nil {
		return errors.Wrap(err, "cleaning up IPAddressClaims")
	}

	// Third cleanup cluster modules.
	if err := janitor.deleteVSphereClusterModules(ctx); err != nil {
		return errors.Wrap(err, "cleaning up vSphere cluster modules")
	}

	return nil
}
