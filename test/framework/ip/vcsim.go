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
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

var _ AddressManager = &vcsim{}

type vcsim struct {
	labels      map[string]string
	client      client.Client
	skipCleanup bool
}

// VCSIMAddressManager returns an ip.AddressManager implementation that leverage vcsim controller capabilities.
func VCSIMAddressManager(client client.Client, labels map[string]string, skipCleanup bool) (AddressManager, error) {
	return &vcsim{
		labels:      labels,
		client:      client,
		skipCleanup: skipCleanup,
	}, nil
}

func (h *vcsim) ClaimIPs(ctx context.Context, opts ...ClaimOption) (AddressClaims, map[string]string) {
	options := &claimOptions{}
	for _, o := range opts {
		o(options)
	}

	variables := map[string]string{}
	ipAddressClaims := AddressClaims{}

	// Claim an IP per variable.
	// NOTE: the code calling this method assumes ControlPlaneEndpointIP is the first claim in the list.
	for _, variable := range append([]string{ControlPlaneEndpointIPVariable}, options.additionalIPVariableNames...) {
		ip, port, ipAddressClaim, err := h.claimIPAddress(ctx)
		Expect(err).ToNot(HaveOccurred())
		ipAddressClaims = append(ipAddressClaims, AddressClaim{
			Namespace: ipAddressClaim.Namespace,
			Name:      ipAddressClaim.Name,
		})
		Byf("Setting clusterctl variable %s to %s", variable, ip)
		variables[variable] = ip

		// All the vcsim controlPlaneEndpoints share the same ip, but have a different port,
		// that we need to pass back as an additional variable.
		// For the CONTROL_PLANE_ENDPOINT_IP variable, we are using the corresponding CONTROL_PLANE_ENDPOINT_PORT variable;
		// for other variable names, we do a best effort replace of the _IP suffix with _PORT, or fail if there is no _IP suffix.
		if variable == ControlPlaneEndpointIPVariable {
			variables[controlPlaneEndpointPortVariable] = port
		} else {
			if !strings.HasSuffix(variable, "_IP") {
				// might be we want to shift to a better error management here, but for now this should be enough to point in the right direction
				panic(fmt.Sprintf("unable to claim vcsim controlPlaneEndpoint for variable name %s. variable name must end with _IP", variables))
			}
			variables[strings.Replace(variable, "_IP", "_PORT", -1)] = port
		}
	}

	return ipAddressClaims, variables
}

func (h *vcsim) Cleanup(ctx context.Context, ipAddressClaims AddressClaims) error {
	if CurrentSpecReport().Failed() {
		By("Skipping cleanup of vcsim ControlPlaneEndpoint because the tests failed and the IPs could still be in use")
		return nil
	}

	if h.skipCleanup {
		By("Skipping cleanup of vcsim ControlPlaneEndpoint because skipCleanup is set to true")
		return nil
	}

	var errList []error

	for _, ipAddressClaim := range ipAddressClaims {
		controlPlaneEndpoint := &vcsimv1.ControlPlaneEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ipAddressClaim.Name,
				Namespace: ipAddressClaim.Namespace,
			},
		}
		Byf("Deleting vcsim ControlPlaneEndpoint %s", klog.KObj(controlPlaneEndpoint))
		if err := h.client.Delete(ctx, controlPlaneEndpoint); err != nil && !apierrors.IsNotFound(err) {
			errList = append(errList, err)
		}
	}

	if len(errList) > 0 {
		return kerrors.NewAggregate(errList)
	}
	return nil
}

// Teardown lists all ControlPlaneEndpoint matching the passed labels and deletes them.
func (h *vcsim) Teardown(ctx context.Context, _ ...TearDownOption) error {
	if h.skipCleanup {
		By("Skipping cleanup of vcsim ControlPlaneEndpoints because skipCleanup is set to true")
		return nil
	}

	// List all ControlPlaneEndpoint created matching the labels.
	controlPlaneEndpoints := &vcsimv1.ControlPlaneEndpointList{}
	if err := h.client.List(ctx, controlPlaneEndpoints,
		client.MatchingLabels(h.labels),
		client.InNamespace(metav1.NamespaceDefault),
	); err != nil {
		return err
	}

	ipAddressClaimsToDelete := AddressClaims{}
	// Collect errors and skip these ip address claims, but report at the end.
	for _, controlPlaneEndpoint := range controlPlaneEndpoints.Items {
		ipAddressClaimsToDelete = append(ipAddressClaimsToDelete, AddressClaim{
			Namespace: controlPlaneEndpoint.Namespace,
			Name:      controlPlaneEndpoint.Name,
		})
	}
	return h.Cleanup(ctx, ipAddressClaimsToDelete)
}

func (h *vcsim) claimIPAddress(ctx context.Context) (_, _ string, _ *vcsimv1.ControlPlaneEndpoint, err error) {
	controlPlaneEndpoint := &vcsimv1.ControlPlaneEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "ipclaim-" + rand.String(32),
			Namespace:   metav1.NamespaceDefault,
			Labels:      h.labels,
			Annotations: map[string]string{},
		},
		Spec: vcsimv1.ControlPlaneEndpointSpec{},
	}

	// Set job name as annotation if environment variable is set.
	if val := os.Getenv("JOB_NAME"); val != "" {
		controlPlaneEndpoint.ObjectMeta.Annotations["prow.k8s.io/job"] = val
	}

	// Create a ControlPlaneEndpoint
	Byf("Creating vcsim ControlPlaneEndpoint %s", klog.KObj(controlPlaneEndpoint))
	if err := h.client.Create(ctx, controlPlaneEndpoint); err != nil {
		return "", "", nil, err
	}

	var retryError error
	// Wait for the controlPlaneEndpoint to report an IPAddress.
	_ = wait.PollUntilContextTimeout(ctx, time.Second, time.Second*30, true, func(ctx context.Context) (done bool, err error) {
		if err := h.client.Get(ctx, client.ObjectKeyFromObject(controlPlaneEndpoint), controlPlaneEndpoint); err != nil {
			retryError = errors.Wrap(err, "getting vcsim ControlPlaneEndpoint")
			return false, nil
		}

		if controlPlaneEndpoint.Status.Host == "" {
			retryError = errors.New("vcsim ControlPlaneEndpoint.Status.Host is not set")
			return false, nil
		}

		retryError = nil
		return true, nil
	})
	if retryError != nil {
		// Try best effort deletion of the unused controlPlaneEndpoint before returning an error.
		_ = h.client.Delete(ctx, controlPlaneEndpoint)
		return "", "", nil, retryError
	}

	return controlPlaneEndpoint.Status.Host, strconv.Itoa(int(controlPlaneEndpoint.Status.Port)), controlPlaneEndpoint, nil
}
