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

package ip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	ipamScheme = runtime.NewScheme()
	_ = ipamv1.AddToScheme(ipamScheme)
}

var _ AddressManager = &inCluster{}

type inCluster struct {
	client      client.Client
	labels      map[string]string
	skipCleanup bool
}

// InClusterAddressManager returns an ip.AddressManager implementation that leverage on the IPAM provider installed into the management cluster.
// If e2eIPAMKubeconfig is an empty string it will return a noop AddressManager which does nothing so we can fallback on setting environment variables.
func InClusterAddressManager(e2eIPAMKubeconfig string, labels map[string]string, skipCleanup bool) (AddressManager, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("expecting labels to be set to prevent deletion of other IPAddressClaims")
	}

	if e2eIPAMKubeconfig == "" {
		return &noop{}, nil
	}

	ipamClient, err := getClient(e2eIPAMKubeconfig)
	if err != nil {
		return nil, err
	}

	return &inCluster{
		labels:      labels,
		client:      ipamClient,
		skipCleanup: skipCleanup,
	}, nil
}

func (h *inCluster) ClaimIPs(ctx context.Context, opts ...ClaimOption) (AddressClaims, map[string]string) {
	options := &claimOptions{}
	for _, o := range opts {
		o(options)
	}

	variables := map[string]string{}
	ipAddressClaims := AddressClaims{}

	// Claim an IP per variable.
	for _, variable := range append(options.additionalIPVariableNames, ControlPlaneEndpointIPVariable) {
		ip, ipAddressClaim, err := h.claimIPAddress(ctx)
		Expect(err).ToNot(HaveOccurred())
		ipAddressClaims = append(ipAddressClaims, AddressClaim{
			Namespace: ipAddressClaim.Namespace,
			Name:      ipAddressClaim.Name,
		})
		Byf("Setting clusterctl variable %s to %s", variable, ip.Spec.Address)
		variables[variable] = ip.Spec.Address
		if variable == ControlPlaneEndpointIPVariable && options.gatewayIPVariableName != "" {
			// Set the gateway variable if requested to the gateway of the control plane IP.
			// This is required in ipam scenarios, otherwise the VMs will not be able to
			// connect to the public internet to pull images.
			Byf("Setting clusterctl variable %s to %s", variable, ip.Spec.Gateway)
			variables[options.gatewayIPVariableName] = ip.Spec.Gateway
		}
	}

	return ipAddressClaims, variables
}

// Cleanup deletes the IPAddressClaims passed.
func (h *inCluster) Cleanup(ctx context.Context, ipAddressClaims AddressClaims) error {
	if CurrentSpecReport().Failed() {
		By("Skipping cleanup of IPAddressClaims because the tests failed and the IPs could still be in use")
		return nil
	}

	if h.skipCleanup {
		By("Skipping cleanup of IPAddressClaims because skipCleanup is set to true")
		return nil
	}

	var errList []error

	for _, ipAddressClaim := range ipAddressClaims {
		claim := &ipamv1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ipAddressClaim.Name,
				Namespace: ipAddressClaim.Namespace,
			},
		}
		Byf("Deleting IPAddressClaim %s", klog.KObj(claim))
		if err := h.client.Delete(ctx, claim); err != nil {
			errList = append(errList, err)
		}
	}

	if len(errList) > 0 {
		return kerrors.NewAggregate(errList)
	}
	return nil
}

// GetIPAddressClaimLabels returns a labels map from the prow environment variables
// BUILD_ID and JOB_NAME. If none of both is set it falls back to add a custom random
// label.
func GetIPAddressClaimLabels() map[string]string {
	labels := map[string]string{}
	if val := os.Getenv("BUILD_ID"); val != "" {
		labels["prow.k8s.io/build-id"] = val
	}
	if val := os.Getenv("REPO_NAME"); val != "" {
		labels["prow.k8s.io/repo-name"] = val
	}
	if len(labels) == 0 {
		// Adding a custom label so we don't accidentally cleanup other IPAddressClaims
		labels["capv-testing/random-uid"] = rand.String(32)
	}
	return labels
}

// Teardown lists all IPAddressClaims matching the passed labels and deletes the IPAddressClaim
// if there are no VirtualMachines in vCenter using the IP address.
func (h *inCluster) Teardown(ctx context.Context, opts ...TearDownOption) error {
	options := &teardownOptions{}
	for _, o := range opts {
		o(options)
	}

	if h.skipCleanup {
		By("Skipping cleanup of IPAddressClaims because skipCleanup is set to true")
		return nil
	}

	virtualMachineIPAddresses, err := getVirtualMachineIPAddresses(ctx, options.folderName, options.vSphereClient)
	if err != nil {
		return err
	}
	// List all IPAddressClaims created matching the labels.
	ipAddressClaims := &ipamv1.IPAddressClaimList{}
	if err := h.client.List(ctx, ipAddressClaims,
		client.MatchingLabels(h.labels),
		client.InNamespace(metav1.NamespaceDefault),
	); err != nil {
		return err
	}

	ipAddressClaimsToDelete := AddressClaims{}
	// Collect errors and skip these ip address claims, but report at the end.
	var errList []error

	ip := &ipamv1.IPAddress{}
	for _, ipAddressClaim := range ipAddressClaims.Items {
		ipAddressClaim := ipAddressClaim
		if ipAddressClaim.Status.AddressRef.Name == "" {
			continue
		}
		if err := h.client.Get(ctx, client.ObjectKey{Namespace: ipAddressClaim.GetNamespace(), Name: ipAddressClaim.Status.AddressRef.Name}, ip); err != nil {
			// If we are not able to get an IP Address we skip the deletion for it but collect and return the error.
			errList = append(errList, errors.Wrapf(err, "getting IPAddress for IPAddressClaim %s", klog.KObj(&ipAddressClaim)))
			continue
		}

		// Skip deletion if there is still a virtual machine which refers this IP address.
		if virtualMachineIPAddresses[ip.Spec.Address] {
			continue
		}

		ipAddressClaimsToDelete = append(ipAddressClaimsToDelete, AddressClaim{
			Namespace: ipAddressClaim.Namespace,
			Name:      ipAddressClaim.Name,
		})
	}

	if err := h.Cleanup(ctx, ipAddressClaimsToDelete); err != nil {
		// Group with possible previous errors.
		errList = append(errList, err)
	}

	if len(errList) > 0 {
		return kerrors.NewAggregate(errList)
	}
	return nil
}

func getClient(e2eIPAMKubeconfig string) (client.Client, error) {
	kubeConfig, err := os.ReadFile(filepath.Clean(e2eIPAMKubeconfig))
	if err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	return client.New(restConfig, client.Options{Scheme: ipamScheme})
}

// getVirtualMachineIPAddresses lists all VirtualMachines in the given folder and
// returns a map which contains the IP addresses of all machines.
// If the given folder is not found it will return an error.
func getVirtualMachineIPAddresses(ctx context.Context, folderName string, vSphereClient *govmomi.Client) (map[string]bool, error) {
	finder := find.NewFinder(vSphereClient.Client)

	// Find the given folder.
	folder, err := finder.FolderOrDefault(ctx, folderName)
	if err != nil {
		return nil, errors.Wrap(err, "getting default folder")
	}

	// List all VirtualMachines in the folder.
	managedObjects, err := finder.ManagedObjectListChildren(ctx, folder.InventoryPath+"/...", "VirtualMachine")
	if err != nil {
		return nil, errors.Wrap(err, "finding VirtualMachines")
	}

	var vm mo.VirtualMachine
	virtualMachineIPAddresses := map[string]bool{}

	// Iterate over the VirtualMachines, get the `guest.net` property and extract the IP addresses.
	for _, mobj := range managedObjects {
		// Get guest.net properties for mobj.
		if err := vSphereClient.RetrieveOne(ctx, mobj.Object.Reference(), []string{"guest.net"}, &vm); err != nil {
			// We cannot get the properties e.g. when the machine already got deleted or is getting deleted.
			Byf("Ignoring VirtualMachine %s during ipam Teardown due to error retrieving properties: %v", mobj.Path, err)
			continue
		}
		// Iterate over all nics and add IP addresses to virtualMachineIPAddresses.
		for _, nic := range vm.Guest.Net {
			if nic.IpConfig == nil {
				continue
			}
			for _, ip := range nic.IpConfig.IpAddress {
				virtualMachineIPAddresses[ip.IpAddress] = true
			}
		}
	}

	return virtualMachineIPAddresses, nil
}

func (h *inCluster) claimIPAddress(ctx context.Context) (_ *ipamv1.IPAddress, _ *ipamv1.IPAddressClaim, err error) {
	claim := &ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "ipclaim-" + rand.String(32),
			Namespace:   metav1.NamespaceDefault,
			Labels:      h.labels,
			Annotations: map[string]string{},
		},
		Spec: ipamv1.IPAddressClaimSpec{
			PoolRef: corev1.TypedLocalObjectReference{
				APIGroup: ptr.To("ipam.cluster.x-k8s.io"),
				Kind:     "InClusterIPPool",
				Name:     "capv-e2e-ippool",
			},
		},
	}
	// Set job name as annotation if environment variable is set.
	if val := os.Getenv("JOB_NAME"); val != "" {
		claim.ObjectMeta.Annotations["prow.k8s.io/job"] = val
	}

	// Create an IPAddressClaim
	Byf("Creating IPAddressClaim %s", klog.KObj(claim))
	if err := h.client.Create(ctx, claim); err != nil {
		return nil, nil, err
	}
	// Store claim inside the service so the cleanup function knows what to delete.
	ip := &ipamv1.IPAddress{}

	var retryError error
	// Wait for the IPAddressClaim to refer an IPAddress.
	_ = wait.PollUntilContextTimeout(ctx, time.Second, time.Second*30, true, func(ctx context.Context) (done bool, err error) {
		if err := h.client.Get(ctx, client.ObjectKeyFromObject(claim), claim); err != nil {
			retryError = errors.Wrap(err, "getting IPAddressClaim")
			return false, nil
		}

		if claim.Status.AddressRef.Name == "" {
			retryError = errors.New("IPAddressClaim.Status.AddressRef.Name is not set")
			return false, nil
		}

		if err := h.client.Get(ctx, client.ObjectKey{Namespace: claim.GetNamespace(), Name: claim.Status.AddressRef.Name}, ip); err != nil {
			retryError = errors.Wrap(err, "getting IPAddress")
			return false, nil
		}
		if ip.Spec.Address == "" {
			retryError = errors.New("IPAddress.Spec.Address is not set")
			return false, nil
		}

		retryError = nil
		return true, nil
	})
	if retryError != nil {
		// Try best effort deletion of the unused claim before returning an error.
		_ = h.client.Delete(ctx, claim)
		return nil, nil, retryError
	}

	return ip, claim, nil
}
