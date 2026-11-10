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

package net_test

import (
	"testing"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/net"
)

func TestErrOnLocalOnlyIPAddr(t *testing.T) {
	testCases := []struct {
		name      string
		ipAddr    string
		expectErr bool
	}{
		{
			name:      "valid-ipv4",
			ipAddr:    "192.168.2.33",
			expectErr: false,
		},
		{
			name:      "valid-ipv6",
			ipAddr:    "1200:0000:AB00:1234:0000:2552:7777:1313",
			expectErr: false,
		},
		{
			name:      "localhost",
			ipAddr:    "127.0.0.1",
			expectErr: true,
		},
		{
			name:      "link-local-unicast-ipv4",
			ipAddr:    "169.254.2.3",
			expectErr: true,
		},
		{
			name:      "link-local-unicast-ipv6",
			ipAddr:    "fe80::250:56ff:feb0:345d",
			expectErr: true,
		},
		{
			name:      "link-local-multicast-ipv4",
			ipAddr:    "224.0.0.252",
			expectErr: true,
		},
		{
			name:      "link-local-multicast-ipv6",
			ipAddr:    "FF02:0:0:0:0:0:1:3",
			expectErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := net.ErrOnLocalOnlyIPAddr(tc.ipAddr); err != nil {
				t.Log(err)
				if !tc.expectErr {
					t.Fatal(err)
				}
			} else if tc.expectErr {
				t.Fatal("expected error did not occur")
			}
		})
	}
}
