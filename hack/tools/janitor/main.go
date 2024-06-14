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

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/pkg/boskos"
	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/pkg/janitor"
)

var ipamScheme *runtime.Scheme

func init() {
	ipamScheme = runtime.NewScheme()
	_ = ipamv1.AddToScheme(ipamScheme)
}

var (
	dryRun        bool
	ipamNamespace string
	maxAge        time.Duration
	// Flags to get folders and resource pools from Boskos.
	boskosHost    string
	resourceOwner string
	resourceTypes []string
	// Flags to directly specify folders and resource pools.
	vsphereVMFolders     []string
	vsphereFolders       []string
	vsphereResourcePools []string
)

func initFlags(fs *pflag.FlagSet) {
	// Note: Intentionally not adding a fallback value, so it is still possible to not use Boskos.
	fs.StringVar(&boskosHost, "boskos-host", os.Getenv("BOSKOS_HOST"), "Boskos server URL. Boskos is only used to retrieve resources if this flag is set.")
	fs.StringVar(&resourceOwner, "resource-owner", "vsphere-janitor", "Owner for the resource.")
	fs.StringArrayVar(&resourceTypes, "resource-type", []string{"vsphere-project-cluster-api-provider", "vsphere-project-cloud-provider", "vsphere-project-image-builder"}, "Types of the resources")
	fs.StringArrayVar(&vsphereVMFolders, "vm-folder", []string{}, "Path to folders in vCenter to cleanup virtual machines.")
	fs.StringArrayVar(&vsphereFolders, "folder", []string{}, "Path to a folder in vCenter to recursively cleanup empty subfolders.")
	fs.StringArrayVar(&vsphereResourcePools, "resource-pool", []string{}, "Path to a resource pool in vCenter to recursively cleanup empty child resource pools.")
	fs.StringVar(&ipamNamespace, "ipam-namespace", "", "Namespace for IPAddressClaim cleanup.")
	fs.DurationVar(&maxAge, "max-age", time.Hour*12, "Maximum age of an object before it is getting deleted.")
	fs.BoolVar(&dryRun, "dry-run", false, "dry-run results in not deleting anything but printing the actions.")
}

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	log := klog.Background()
	// Just setting this to avoid that CR is complaining about a missing logger.
	ctrl.SetLogger(log)
	ctx := ctrl.LoggerInto(context.Background(), log)

	if err := run(ctx); err != nil {
		log.Error(err, "Failed running vsphere-janitor")
		os.Exit(1)
	}

	log.Info("Finished cleanup.")
}

func run(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)

	// Create clients for vSphere.
	vSphereClients, err := janitor.NewVSphereClients(ctx, janitor.NewVSphereClientsInput{
		Username:   os.Getenv("GOVC_USERNAME"),
		Password:   os.Getenv("GOVC_PASSWORD"),
		Server:     os.Getenv("GOVC_URL"),
		Thumbprint: os.Getenv("VSPHERE_TLS_THUMBPRINT"),
		UserAgent:  "capv-janitor",
	})
	if err != nil {
		return errors.Wrap(err, "creating vSphere clients")
	}
	defer vSphereClients.Logout(ctx)

	// Create controller-runtime client for IPAM.
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "unable to get kubeconfig")
	}
	ipamClient, err := client.New(restConfig, client.Options{Scheme: ipamScheme})
	if err != nil {
		return errors.Wrap(err, "creating IPAM client")
	}

	if boskosHost != "" {
		log = log.WithValues("boskosHost", boskosHost, "resourceOwner", resourceOwner)
		log.Info("Getting resources to cleanup from Boskos")
		client, err := boskos.NewClient(resourceOwner, boskosHost)
		if err != nil {
			return err
		}

		var allErrs []error
		for _, resourceType := range resourceTypes {
			// For all resource in state dirty that are currently not owned:
			// * acquire the resource (and set it to state "cleaning")
			// * try to clean up vSphere
			// * if cleanup succeeds, release the resource as free
			// * if cleanup fails, resource will stay in cleaning and become stale (reaper will move it to dirty)
			for {
				log.Info("Acquiring resource")
				res, err := client.Acquire(resourceType, boskos.Dirty, boskos.Cleaning)
				if err != nil {
					// If we get an error on acquire we're done looping through all dirty resources
					if errors.Is(err, boskos.ErrNotFound) {
						// Note: ErrNotFound means there are no more dirty resources that are not owned.
						log.Info("No more resources to cleanup")
						break
					}
					allErrs = append(allErrs, errors.Wrapf(err, "failed to acquire resource"))
					break
				}
				log := log.WithValues("resourceName", res.Name)

				if res.UserData == nil {
					allErrs = append(allErrs, errors.Errorf("failed to get user data, resource %q is missing user data", res.Name))
					continue
				}

				folder, hasFolder := res.UserData.Load("folder")
				if !hasFolder {
					allErrs = append(allErrs, errors.Errorf("failed to get user data, resource %q is missing \"folder\" key", res.Name))
					continue
				}
				resourcePool, hasResourcePool := res.UserData.Load("resourcePool")
				if !hasResourcePool {
					allErrs = append(allErrs, errors.Errorf("failed to get user data, resource %q is missing \"resourcePool\" key", res.Name))
					continue
				}

				// Delete all VMs created up until now.
				maxCreationDate := time.Now()
				j := janitor.NewJanitor(vSphereClients, nil, maxCreationDate, "", false)

				log.Info("Cleaning up vSphere")
				if err := j.CleanupVSphere(ctx, []string{folder.(string)}, []string{resourcePool.(string)}, []string{folder.(string)}, false); err != nil {
					log.Info("Cleaning up vSphere failed")

					// Intentionally keep this resource in cleaning state. The reaper will move it from cleaning to dirty
					// and we'll retry the cleanup.
					// If we move it to dirty here, the for loop will pick it up again, and we get stuck in an infinite loop.
					allErrs = append(allErrs, errors.Wrapf(err, "cleaning up vSphere failed, resource %q will now become stale", res.Name))
					continue
				}
				log.Info("Cleaning up vSphere succeeded")

				// Try to release resource as free.
				log.Info("Releasing resource as free")
				if releaseErr := client.Release(res.Name, boskos.Free); releaseErr != nil {
					allErrs = append(allErrs, errors.Wrapf(releaseErr, "cleaning up vSphere succeeded and releasing resource as free failed, resource %q will now become stale", res.Name))
				}
				log.Info("Releasing resource as free succeeded")
			}
		}
		if len(allErrs) > 0 {
			return errors.Wrap(kerrors.NewAggregate(allErrs), "cleaning up Boskos resources")
		}
	}

	// Note: The following will be deleted once we migrated all repos to Boskos.
	maxCreationDate := time.Now().Add(-maxAge)
	janitor := janitor.NewJanitor(vSphereClients, ipamClient, maxCreationDate, ipamNamespace, dryRun)

	log.Info("Configured settings", "dry-run", dryRun)
	log.Info("Configured settings", "folders", vsphereFolders)
	log.Info("Configured settings", "vm-folders", vsphereVMFolders)
	log.Info("Configured settings", "resource-pools", vsphereResourcePools)
	log.Info("Configured settings", "ipam-namespace", ipamNamespace)
	log.Info("Configured settings", "max-age", maxAge)
	log.Info("Configured settings", "janitor.maxCreationDate", maxCreationDate)

	// First cleanup old vms and other vSphere resources to free up IPAddressClaims or cluster modules which are still in-use.
	if err := janitor.CleanupVSphere(ctx, vsphereFolders, vsphereResourcePools, vsphereVMFolders, false); err != nil {
		return errors.Wrap(err, "cleaning up vSphere")
	}

	// Second cleanup IPAddressClaims.
	if err := janitor.DeleteIPAddressClaims(ctx); err != nil {
		return errors.Wrap(err, "cleaning up IPAddressClaims")
	}

	return nil
}
