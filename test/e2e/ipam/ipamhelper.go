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

// Package ipam is a helper to claim ip addresses from an IPAM provider cluster.
package ipam

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	. "sigs.k8s.io/cluster-api-provider-vsphere/test/e2e/helper"
)

var ipamScheme *runtime.Scheme

const controlPlaneEndpointVariable = "CONTROL_PLANE_ENDPOINT_IP"

func init() {
	ipamScheme = runtime.NewScheme()
	_ = ipamv1.AddToScheme(ipamScheme)
}

type IPAddressClaims []*ipamv1.IPAddressClaim

type Helper interface {
	// ClaimIPs claims IP addresses with the variable name `CONTROL_PLANE_ENDPOINT_IP` and whatever is passed as
	// additionalIPVariableNames and creates a new clusterctl config file.
	// It returns the path to the new clusterctl config file and a slice of IPAddressClaims.
	ClaimIPs(ctx context.Context, clusterctlConfigPath string, additionalIPVariableNames ...string) (localClusterctlConfigFile string, claims IPAddressClaims)

	// Cleanup deletes the given IPAddressClaims.
	Cleanup(ctx context.Context, claims IPAddressClaims) error

	// Teardown tries to cleanup orphaned IPAddressClaims by checking if the corresponding IPs are still in use in vSphere.
	// It identifies IPAddressClaims via labels.
	Teardown(ctx context.Context, folderName string, vSphereClient *govmomi.Client) error
}

// New returns an ipam.Helper. If e2eIPAMKubeconfig is an empty string or skipCleanup is true
// it will return a noop helper which does nothing so we can fallback on setting environment variables.
func New(e2eIPAMKubeconfig string, labels map[string]string, skipCleanup bool) (Helper, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("expecting labels to be set to prevent deletion of other IPAddressClaims")
	}

	if e2eIPAMKubeconfig == "" {
		return &noopHelper{}, nil
	}

	ipamClient, err := getClient(e2eIPAMKubeconfig)
	if err != nil {
		return nil, err
	}

	return &helper{
		labels:      labels,
		client:      ipamClient,
		skipCleanup: skipCleanup,
	}, nil
}

type helper struct {
	client      client.Client
	labels      map[string]string
	skipCleanup bool
}

func (h *helper) ClaimIPs(ctx context.Context, clusterctlConfigPath string, additionalIPVariableNames ...string) (string, IPAddressClaims) {
	variables := map[string]string{}

	ipAddressClaims := []*ipamv1.IPAddressClaim{}

	// Claim an IP per variable.
	for _, variable := range append(additionalIPVariableNames, controlPlaneEndpointVariable) {
		ip, ipAddressClaim, err := h.claimIPAddress(ctx)
		Expect(err).ToNot(HaveOccurred())
		ipAddressClaims = append(ipAddressClaims, ipAddressClaim)
		Byf("Setting clusterctl variable %s to %s", variable, ip)
		variables[variable] = ip
	}

	// Create a new clusterctl config file based on the passed file and add the new variables for the IPs.
	modifiedClusterctlConfigPath := fmt.Sprintf("%s-%s.yaml", strings.TrimSuffix(clusterctlConfigPath, ".yaml"), rand.String(16))
	Byf("Writing a new clusterctl config to %s", modifiedClusterctlConfigPath)
	copyAndAmendClusterctlConfig(ctx, copyAndAmendClusterctlConfigInput{
		ClusterctlConfigPath: clusterctlConfigPath,
		OutputPath:           modifiedClusterctlConfigPath,
		Variables:            variables,
	})

	return modifiedClusterctlConfigPath, ipAddressClaims
}

// Cleanup deletes the IPAddressClaims passed.
func (h *helper) Cleanup(ctx context.Context, ipAddressClaims IPAddressClaims) error {
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
		ipAddressClaim := ipAddressClaim
		Byf("Deleting IPAddressClaim %s", klog.KObj(ipAddressClaim))
		if err := h.client.Delete(ctx, ipAddressClaim); err != nil {
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
	if val := os.Getenv("JOB_NAME"); val != "" {
		labels["prow.k8s.io/job"] = val
	}
	if len(labels) == 0 {
		// Adding a custom label so we don't accidentally cleanup other IPAddressClaims
		labels["capv-testing/random-uid"] = rand.String(32)
	}
	return labels
}

// Teardown lists all IPAddressClaims matching the passed labels and deletes the IPAddressClaim
// if there are no VirtualMachines in vCenter using the IP address.
func (h *helper) Teardown(ctx context.Context, folderName string, vSphereClient *govmomi.Client) error {
	if h.skipCleanup {
		By("Skipping cleanup of IPAddressClaims because skipCleanup is set to true")
		return nil
	}

	virtualMachineIPAddresses, err := getVirtualMachineIPAddresses(ctx, folderName, vSphereClient)
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

	ipAddressClaimsToDelete := []*ipamv1.IPAddressClaim{}
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

		ipAddressClaimsToDelete = append(ipAddressClaimsToDelete, &ipAddressClaim)
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
		return nil, err
	}

	// List all VirtualMachines in the folder.
	managedObjects, err := finder.ManagedObjectListChildren(ctx, folder.InventoryPath+"/...", "VirtualMachine")
	if err != nil {
		return nil, err
	}

	var vm mo.VirtualMachine
	virtualMachineIPAddresses := map[string]bool{}

	// Iterate over the VirtualMachines, get the `guest.net` property and extract the IP addresses.
	for _, mobj := range managedObjects {
		// Get guest.net properties for mobj.
		if err := vSphereClient.RetrieveOne(ctx, mobj.Object.Reference(), []string{"guest.net"}, &vm); err != nil {
			return nil, errors.Wrapf(err, "get properties of VM %s", mobj.Object.Reference())
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

func (h *helper) claimIPAddress(ctx context.Context) (_ string, _ *ipamv1.IPAddressClaim, err error) {
	claim := &ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ipclaim-" + rand.String(32),
			Namespace: metav1.NamespaceDefault,
			Labels:    h.labels,
		},
		Spec: ipamv1.IPAddressClaimSpec{
			PoolRef: corev1.TypedLocalObjectReference{
				APIGroup: ptr.To("ipam.cluster.x-k8s.io"),
				Kind:     "InClusterIPPool",
				Name:     "capv-e2e-ippool",
			},
		},
	}

	// Create an IPAddressClaim
	Byf("Creating IPAddressClaim %s", klog.KObj(claim))
	if err := h.client.Create(ctx, claim); err != nil {
		return "", nil, err
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
			retryError = errors.Wrap(err, "IPAddressClaim.Status.AddressRef.Name is not set")
			return false, nil
		}

		if err := h.client.Get(ctx, client.ObjectKey{Namespace: claim.GetNamespace(), Name: claim.Status.AddressRef.Name}, ip); err != nil {
			retryError = errors.Wrap(err, "getting IPAddress")
			return false, nil
		}
		if ip.Spec.Address == "" {
			retryError = errors.Wrap(err, "IPAddress.Spec.Address is not set")
			return false, nil
		}

		retryError = nil
		return true, nil
	})
	if retryError != nil {
		return "", nil, retryError
	}

	return ip.Spec.Address, claim, nil
}

// Note: Copy-paste from CAPI below.

// copyAndAmendClusterctlConfigInput is the input for copyAndAmendClusterctlConfig.
type copyAndAmendClusterctlConfigInput struct {
	ClusterctlConfigPath string
	OutputPath           string
	Variables            map[string]string
}

// copyAndAmendClusterctlConfig copies the clusterctl-config from ClusterctlConfigPath to
// OutputPath and adds the given Variables.
func copyAndAmendClusterctlConfig(_ context.Context, input copyAndAmendClusterctlConfigInput) {
	// Read clusterctl config from ClusterctlConfigPath.
	clusterctlConfigFile := &clusterctlConfig{
		Path: input.ClusterctlConfigPath,
	}
	clusterctlConfigFile.read()

	// Overwrite variables.
	if clusterctlConfigFile.Values == nil {
		clusterctlConfigFile.Values = map[string]interface{}{}
	}
	for key, value := range input.Variables {
		clusterctlConfigFile.Values[key] = value
	}

	// Write clusterctl config to OutputPath.
	clusterctlConfigFile.Path = input.OutputPath
	clusterctlConfigFile.write()
}

type clusterctlConfig struct {
	Path   string
	Values map[string]interface{}
}

// write writes a clusterctl config file to disk.
func (c *clusterctlConfig) write() {
	data, err := yaml.Marshal(c.Values)
	Expect(err).ToNot(HaveOccurred(), "Failed to marshal the clusterctl config file")

	Expect(os.WriteFile(c.Path, data, 0600)).To(Succeed(), "Failed to write the clusterctl config file")
}

// read reads a clusterctl config file from disk.
func (c *clusterctlConfig) read() {
	data, err := os.ReadFile(c.Path)
	Expect(err).ToNot(HaveOccurred())

	err = yaml.Unmarshal(data, &c.Values)
	Expect(err).ToNot(HaveOccurred(), "Failed to unmarshal the clusterctl config file")
}
