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

	. "github.com/onsi/ginkgo/v2"
	"github.com/vmware/govmomi"
)

type noopHelper struct{}

func (h *noopHelper) ClaimIPs(_ context.Context, _ string, _ ...string) (string, IPAddressClaims) {
	return "", nil
}

func (h *noopHelper) Cleanup(_ context.Context, _ IPAddressClaims) error {
	By("Skipping cleanup of IPAddressClaims because of using ipam.noopHelper")
	return nil
}

func (*noopHelper) Teardown(_ context.Context, _ string, _ *govmomi.Client) error {
	By("Skipping teardown of IPAddressClaims because of using ipam.noopHelper")
	return nil
}
