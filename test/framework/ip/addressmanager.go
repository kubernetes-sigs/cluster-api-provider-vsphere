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

// Package ip provide helpers for ip address management if a vCenter cluster.
package ip

import (
	"context"

	"github.com/vmware/govmomi"
	"k8s.io/apimachinery/pkg/types"
)

type AddressClaims []types.NamespacedName

type AddressManager interface {
	// ClaimIPs claims IP addresses with the variable name `CONTROL_PLANE_ENDPOINT_IP` and whatever is passed as
	// additionalIPVariableNames.
	// It returns a slice of IPAddressClaims namespaced names and corresponding variables.
	ClaimIPs(ctx context.Context, opts ...ClaimOption) (claims AddressClaims, variables map[string]string)

	// Cleanup deletes the given IPAddressClaims.
	Cleanup(ctx context.Context, claims AddressClaims) error

	// Teardown tries to cleanup orphaned IPAddressClaims by checking if the corresponding IPs are still in use in vSphere.
	// It identifies IPAddressClaims via labels.
	Teardown(ctx context.Context, folderName string, vSphereClient *govmomi.Client) error
}

type claimOptions struct {
	additionalIPVariableNames []string
	gatewayIPVariableName     string
}

type ClaimOption func(*claimOptions)

// WithIP instructs Setup to allocate another IP and store it into the provided variableName
// NOTE: Setup always allocate an IP for CONTROL_PLANE_ENDPOINT_IP.
func WithIP(variableName ...string) ClaimOption {
	return func(o *claimOptions) {
		o.additionalIPVariableNames = append(o.additionalIPVariableNames, variableName...)
	}
}

// WithGateway instructs Setup to store the Gateway IP from IPAM into the provided variableName.
func WithGateway(variableName string) ClaimOption {
	return func(o *claimOptions) {
		o.gatewayIPVariableName = variableName
	}
}
