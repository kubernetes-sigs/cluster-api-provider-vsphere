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
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go4.org/netipx"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/pkg/boskos"
	"sigs.k8s.io/cluster-api-provider-vsphere/hack/tools/pkg/janitor"
)

var (
	boskosHost           string
	resourceOwner        string
	resourceType         string
	resourceName         string
	vSphereUsername      string
	vSpherePassword      string
	vSphereServer        string
	vSphereTLSThumbprint string
	vSphereFolder        string
	vSphereResourcePool  string
)

func main() {
	log := klog.Background()
	ctx := ctrl.LoggerInto(context.Background(), log)
	// Just setting this to avoid that CR is complaining about a missing logger.
	ctrl.SetLogger(log)

	rootCmd := setupCommands(ctx)

	if err := rootCmd.Execute(); err != nil {
		log.Error(err, "Failed running boskosctl")
		os.Exit(1)
	}
}

func setupCommands(ctx context.Context) *cobra.Command {
	// Root command
	rootCmd := &cobra.Command{
		Use:          "boskosctl",
		SilenceUsage: true,
		Short:        "boskosctl can be used to consume Boskos vSphere resources",
	}
	// Note: http://boskos.test-pods.svc.cluster.local is the URL of the service usually used in k8s.io clusters.
	rootCmd.PersistentFlags().StringVar(&boskosHost, "boskos-host", getOrDefault(os.Getenv("BOSKOS_HOST"), "http://boskos.test-pods.svc.cluster.local"), "Boskos server URL. (can also be set via BOSKOS_HOST env var)")
	rootCmd.PersistentFlags().StringVar(&resourceOwner, "resource-owner", "", "Owner for the resource.")

	// acquire command
	acquireCmd := &cobra.Command{
		Use:  "acquire",
		Args: cobra.NoArgs,
		RunE: runCmd(ctx),
	}
	acquireCmd.PersistentFlags().StringVar(&resourceType, "resource-type", "", "Type of the resource. Should be one of: vsphere-project-cluster-api-provider, vsphere-project-cloud-provider, vsphere-project-image-builder")
	rootCmd.AddCommand(acquireCmd)

	// heartbeat command
	heartbeatCmd := &cobra.Command{
		Use:  "heartbeat",
		Args: cobra.NoArgs,
		RunE: runCmd(ctx),
	}
	heartbeatCmd.PersistentFlags().StringVar(&resourceName, "resource-name", "", "Name of the resource.")
	rootCmd.AddCommand(heartbeatCmd)

	// release command
	releaseCmd := &cobra.Command{
		Use:  "release",
		Args: cobra.NoArgs,
		RunE: runCmd(ctx),
	}
	releaseCmd.PersistentFlags().StringVar(&resourceName, "resource-name", "", "Name of the resource.")
	releaseCmd.PersistentFlags().StringVar(&vSphereUsername, "vsphere-username", "", "vSphere username of the resource, required for cleanup before release (can also be set via VSPHERE_USERNAME env var)")
	releaseCmd.PersistentFlags().StringVar(&vSpherePassword, "vsphere-password", "", "vSphere password of the resource, required for cleanup before release (can also be set via VSPHERE_PASSWORD env var)")
	releaseCmd.PersistentFlags().StringVar(&vSphereServer, "vsphere-server", "", "vSphere server of the resource, required for cleanup before release")
	releaseCmd.PersistentFlags().StringVar(&vSphereTLSThumbprint, "vsphere-tls-thumbprint", "", "vSphere TLS thumbprint of the resource, required for cleanup before release")
	releaseCmd.PersistentFlags().StringVar(&vSphereFolder, "vsphere-folder", "", "vSphere folder of the resource, required for cleanup before release")
	releaseCmd.PersistentFlags().StringVar(&vSphereResourcePool, "vsphere-resource-pool", "", "vSphere resource pool of the resource, required for cleanup before release")
	rootCmd.AddCommand(releaseCmd)

	return rootCmd
}

func getOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

func runCmd(ctx context.Context) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		log := ctrl.LoggerFrom(ctx)

		if boskosHost == "" {
			return fmt.Errorf("--boskos-host must be set")
		}
		if resourceOwner == "" {
			return fmt.Errorf("--resource-owner must be set")
		}
		log = log.WithValues("boskosHost", boskosHost, "resourceOwner", resourceOwner)
		ctx := ctrl.LoggerInto(ctx, log)

		log.Info("Creating new Boskos client")
		client, err := boskos.NewClient(resourceOwner, boskosHost)
		if err != nil {
			return err
		}

		switch cmd.Use {
		case "acquire":
			if resourceType == "" {
				return fmt.Errorf("--resource-type must be set")
			}
			log := log.WithValues("resourceType", resourceType)
			ctx := ctrl.LoggerInto(ctx, log)

			return acquire(ctx, client, resourceType)
		case "heartbeat":
			if resourceName == "" {
				return fmt.Errorf("--resource-name must be set")
			}
			log := log.WithValues("resourceName", resourceName)
			ctx := ctrl.LoggerInto(ctx, log)

			return heartbeat(ctx, client, resourceName)
		case "release":
			if resourceName == "" {
				return fmt.Errorf("--resource-name must be set")
			}
			if vSphereUsername == "" {
				vSphereUsername = os.Getenv("VSPHERE_USERNAME")
			}
			if vSphereUsername == "" {
				return fmt.Errorf("--vsphere-username or VSPHERE_USERNAME env var must be set")
			}
			if vSpherePassword == "" {
				vSpherePassword = os.Getenv("VSPHERE_PASSWORD")
			}
			if vSpherePassword == "" {
				return fmt.Errorf("--vsphere-password or VSPHERE_PASSWORD env var must be set")
			}
			if vSphereServer == "" {
				return fmt.Errorf("--vsphere-server must be set")
			}
			if vSphereTLSThumbprint == "" {
				return fmt.Errorf("--vsphere-tls-thumbprint must be set")
			}
			if vSphereFolder == "" {
				return fmt.Errorf("--vsphere-folder must be set")
			}
			if vSphereResourcePool == "" {
				return fmt.Errorf("--vsphere-resource-pool must be set")
			}

			log := log.WithValues("resourceName", resourceName, "vSphereServer", vSphereServer, "vSphereFolder", vSphereFolder, "vSphereResourcePool", vSphereResourcePool)
			ctx := ctrl.LoggerInto(ctx, log)

			return release(ctx, client, resourceName, vSphereUsername, vSpherePassword, vSphereServer, vSphereTLSThumbprint, vSphereFolder, vSphereResourcePool)
		}

		return nil
	}
}

func acquire(ctx context.Context, client *boskos.Client, resourceType string) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Acquiring resource")
	res, err := client.Acquire(resourceType, boskos.Free, boskos.Busy)
	if err != nil {
		return errors.Wrapf(err, "failed to acquire resource of type %s", resourceType)
	}
	log.Info(fmt.Sprintf("Acquired resource %q", res.Name))

	if res.UserData == nil {
		return errors.Errorf("failed to get user data, resource %q is missing user data", res.Name)
	}

	folder, hasFolder := res.UserData.Load("folder")
	if !hasFolder {
		return errors.Errorf("failed to get user data, resource %q is missing \"folder\" key", res.Name)
	}
	resourcePool, hasResourcePool := res.UserData.Load("resourcePool")
	if !hasResourcePool {
		return errors.Errorf("failed to get user data, resource %q is missing \"resourcePool\" key", res.Name)
	}
	ipPool, hasIPPool := res.UserData.Load("ipPool")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("export BOSKOS_RESOURCE_NAME=%s\n", res.Name))
	sb.WriteString(fmt.Sprintf("export BOSKOS_RESOURCE_FOLDER=%s\n", folder))
	sb.WriteString(fmt.Sprintf("export BOSKOS_RESOURCE_POOL=%s\n", resourcePool))

	if hasIPPool {
		envVars, err := getIPPoolEnvVars(ipPool.(string))
		if err != nil {
			return errors.Wrapf(err, "failed to calculate IP pool env vars")
		}
		for k, v := range envVars {
			sb.WriteString(fmt.Sprintf("export %s=%s\n", k, v))
		}
	}

	fmt.Println(sb.String())

	return nil
}

// inClusterIPPoolSpec defines the desired state of InClusterIPPool.
// Note: This is a copy of the relevant fields from: https://github.com/kubernetes-sigs/cluster-api-ipam-provider-in-cluster/blob/main/api/v1alpha2/inclusterippool_types.go
// This was copied to avoid a go dependency on this provider.
type inClusterIPPoolSpec struct {
	// Addresses is a list of IP addresses that can be assigned. This set of
	// addresses can be non-contiguous.
	Addresses []string `json:"addresses"`

	// Prefix is the network prefix to use.
	// +kubebuilder:validation:Maximum=128
	Prefix int `json:"prefix"`

	// Gateway
	// +optional
	Gateway string `json:"gateway,omitempty"`
}

// getIPPoolEnvVars calculates env vars based on the ipPool string.
// Note: It's easier to calculate these env vars here in Go compared to if consumers of boskosctl have to do it in bash.
func getIPPoolEnvVars(ipPool string) (map[string]string, error) {
	ipPoolSpec := inClusterIPPoolSpec{}
	if err := json.Unmarshal([]byte(ipPool), &ipPoolSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal IP pool configuration")
	}

	ipSet, err := allIPs(ipPoolSpec.Addresses)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate IP addresses")
	}

	envVars := map[string]string{
		// We need surrounding '' so the JSON string is preserved correctly.
		"BOSKOS_RESOURCE_IP_POOL":         fmt.Sprintf("'%s'", ipPool),
		"BOSKOS_RESOURCE_IP_POOL_PREFIX":  strconv.Itoa(ipPoolSpec.Prefix),
		"BOSKOS_RESOURCE_IP_POOL_GATEWAY": ipPoolSpec.Gateway,
	}
	for i, ip := range ipSet {
		envVars[fmt.Sprintf("BOSKOS_RESOURCE_IP_POOL_IP_%d", i)] = ip.String()
	}
	return envVars, nil
}

// allIPs gets all IPs from addresses.
// Note: Based on https://github.com/kubernetes-sigs/cluster-api-ipam-provider-in-cluster/blob/f656d1d169aea5063dd2e0563f94ef1cc384371e/internal/poolutil/pool.go#L160
func allIPs(addressesArray []string) ([]netip.Addr, error) {
	builder := &netipx.IPSetBuilder{}

	for _, addresses := range addressesArray {
		if strings.Contains(addresses, "-") {
			addrRange, err := netipx.ParseIPRange(addresses)
			if err != nil {
				return nil, err
			}
			builder.AddRange(addrRange)
		} else if strings.Contains(addresses, "/") {
			prefix, err := netip.ParsePrefix(addresses)
			if err != nil {
				return nil, err
			}
			builder.AddPrefix(prefix)
		} else {
			addr, err := netip.ParseAddr(addresses)
			if err != nil {
				return nil, err
			}
			builder.Add(addr)
		}
	}
	ipSet, err := builder.IPSet()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate IP set from addresses")
	}

	var allIPs []netip.Addr
	for _, ipRange := range ipSet.Ranges() {
		ip := ipRange.From()
		for {
			allIPs = append(allIPs, ip)
			if ip == ipRange.To() {
				break
			}
			ip = ip.Next()
		}
	}
	return allIPs, nil
}

func heartbeat(ctx context.Context, client *boskos.Client, resourceName string) error {
	log := ctrl.LoggerFrom(ctx)
	for {
		log.Info("Sending heartbeat")

		if err := client.Update(resourceName, boskos.Busy, nil); err != nil {
			log.Error(err, "Sending heartbeat failed")
		} else {
			log.Error(err, "Sending heartbeat succeeded")
		}

		time.Sleep(1 * time.Minute)
	}
}

func release(ctx context.Context, client *boskos.Client, resourceName, vSphereUsername, vSpherePassword, vSphereServer, vSphereTLSThumbprint, vSphereFolder, vSphereResourcePool string) error {
	log := ctrl.LoggerFrom(ctx)
	ctx = ctrl.LoggerInto(ctx, log)

	log.Info("Releasing resource")

	// Create clients for vSphere.
	vSphereClients, err := janitor.NewVSphereClients(ctx, janitor.NewVSphereClientsInput{
		Username:   vSphereUsername,
		Password:   vSpherePassword,
		Server:     vSphereServer,
		Thumbprint: vSphereTLSThumbprint,
		UserAgent:  "boskosctl",
	})
	if err != nil {
		return errors.Wrap(err, "failed to create vSphere clients")
	}
	defer vSphereClients.Logout(ctx)

	// Delete all VMs created up until now.
	j := janitor.NewJanitor(vSphereClients, false)

	log.Info("Cleaning up vSphere")
	// Note: We intentionally want to skip clusterModule cleanup. If we run this too often we might hit race conditions
	// when other tests are creating cluster modules in parallel.
	if err := j.CleanupVSphere(ctx, []string{vSphereFolder}, []string{vSphereResourcePool}, []string{vSphereFolder}, true); err != nil {
		log.Info("Cleaning up vSphere failed")

		// Try to release resource as dirty.
		log.Info("Releasing resource as dirty")
		if releaseErr := client.Release(resourceName, boskos.Dirty); releaseErr != nil {
			return errors.Wrapf(kerrors.NewAggregate([]error{err, releaseErr}), "cleaning up vSphere and releasing resource as dirty failed, resource will now become stale")
		}
		log.Info("Releasing resource as dirty succeeded")

		return errors.Wrapf(err, "cleaning up vSphere failed, resource was released as dirty")
	}
	log.Info("Cleaning up vSphere succeeded")

	// Try to release resource as free.
	log.Info("Releasing resource as free")
	if releaseErr := client.Release(resourceName, boskos.Free); releaseErr != nil {
		return errors.Wrapf(releaseErr, "cleaning up vSphere succeeded and releasing resource as free failed, resource will now become stale")
	}
	log.Info("Releasing resource as free succeeded")

	return nil
}
