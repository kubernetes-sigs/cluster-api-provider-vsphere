/*
Copyright 2022 The Kubernetes Authors.

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

package util

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestSanitizeIPHostInfoLabel(t *testing.T) {
	tests := []struct {
		name, input, expected string
	}{
		{
			name:     "for valid IPv4 address",
			input:    "1.2.3.4",
			expected: "1.2.3.4",
		},
		{
			name:     "for valid IPv6 address",
			input:    "2620:124:6020:c003:0:69ff:fe59:80ac",
			expected: "2620-124-6020-c003-0-69ff-fe59-80ac.ipv6-literal",
		},
		{
			name:     "for a shorthand valid IPv6 address",
			input:    "2620::c003:0:69ff:fe59:80ac",
			expected: "2620--c003-0-69ff-fe59-80ac.ipv6-literal",
		},
		{
			name:     "for a valid IPv6 address with zone index",
			input:    "2620:124:6020:c003:0:69ff:fe59:80ac%3",
			expected: "2620-124-6020-c003-0-69ff-fe59-80ac.ipv6-literal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			g.Expect(SanitizeHostInfoLabel(tt.input)).To(gomega.Equal(tt.expected))
		})
	}
}

func TestSanitizeDNSHostInfoLabel(t *testing.T) {
	tests := []struct {
		name, input, expected string
	}{
		{
			name:     "for DNS entry with less than 63 characters",
			input:    "foo-1.bar.com",
			expected: "foo-1.bar.com",
		},
		{
			name:     "for DNS entry with 63 characters",
			input:    "esx13-r09.p01.1d91f0ee4f14b7e83bd42.australiaeast.avs.belch.com",
			expected: "esx13-r09.p01.1d91f0ee4f14b7e83bd42.australiaeast.avs.belch.com",
		},
		{
			name:     "for DNS entry with > 63 characters",
			input:    "esx13-r09.p01.1d91f0ee4f14b7e83bd420.australiaeast.avs.belch.com",
			expected: "esx13-r09.p01.1d91f0ee4f14b7e83bd420.australiaeast.avs.belch",
		},
		{
			name:     "for DNS entry with > 63 characters with multiple segments dropped",
			input:    "esx13-r09.p01.1d91f0ee4f14b7e83bd420.australiaeast.avs.belch.com.us",
			expected: "esx13-r09.p01.1d91f0ee4f14b7e83bd420.australiaeast.avs.belch",
		},
		{
			name:     "for DNS entry with > 63 characters with length = 63 after truncation",
			input:    "esx13-r09.p01.1d91f0ee4f14b7e83bd420az.australiaeast.avs.belch.us",
			expected: "esx13-r09.p01.1d91f0ee4f14b7e83bd420az.australiaeast.avs.belch",
		},
		{
			name:     "for DNS entry with > 63 characters with first segment > 63 characters",
			input:    "esx-zcvU3CecjX8Tr5qXQgztj9ZKCp369p3hLFdzAu8VwEyWGq4hzkLTNZq089TI.p01.1d91f0ee4f14b7e83bd420.australiaeast.avs.belch.com",
			expected: "esx-zcvU3CecjX8Tr5qXQgztj9ZKCp369p3hLFdzAu8VwEyWGq4hzkLTNZq089T",
		},
		{
			name:     "for DNS entry with > 63 characters with first segment = 63 characters",
			input:    "esx-zcvU3CecjX8Tr5qXQgztj9ZKCp369p3hLFdzAu8VwEyWGq4hzkLTNZq089T.p01.1d91f0ee4f14b7e83bd420.australiaeast.avs.belch.com",
			expected: "esx-zcvU3CecjX8Tr5qXQgztj9ZKCp369p3hLFdzAu8VwEyWGq4hzkLTNZq089T",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			g.Expect(SanitizeHostInfoLabel(tt.input)).To(gomega.Equal(tt.expected))
		})
	}
}
