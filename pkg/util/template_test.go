/*
Copyright 2025 The Kubernetes Authors.

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

	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/utils/ptr"
)

func Test_GenerateMachineNameFromTemplate(t *testing.T) {
	tests := []struct {
		name        string
		machineName string
		template    *string
		want        []gomegatypes.GomegaMatcher
		wantErr     bool
	}{
		{
			name:        "return machineName if template is nil",
			machineName: "quick-start-d34gt4-md-0-wqc85-8nxwc-gfd5v",
			template:    nil,
			want: []gomegatypes.GomegaMatcher{
				Equal("quick-start-d34gt4-md-0-wqc85-8nxwc-gfd5v"),
			},
		},
		{
			name:        "template which doesn't respect max length: trim to max length",
			machineName: "quick-start-d34gt4-md-0-wqc85-8nxwc-gfd5v", // 41 characters
			template:    ptr.To[string]("{{ .machine.name }}-{{ .machine.name }}"),
			want: []gomegatypes.GomegaMatcher{
				Equal("quick-start-d34gt4-md-0-wqc85-8nxwc-gfd5v-quick-start-d34gt4-md"), // 63 characters
			},
		},
		{
			name:        "template for 20 characters: keep machine name if name has 20 characters",
			machineName: "quick-md-8nxwc-gfd5v", // 20 characters
			template:    ptr.To[string]("{{ if le (len .machine.name) 20 }}{{ .machine.name }}{{else}}{{ trimSuffix \"-\" (trunc 14 .machine.name) }}-{{ trunc -5 .machine.name }}{{end}}"),
			want: []gomegatypes.GomegaMatcher{
				Equal("quick-md-8nxwc-gfd5v"), // 20 characters
			},
		},
		{
			name:        "template for 20 characters: trim to 20 characters if name has more than 20 characters",
			machineName: "quick-start-d34gt4-md-0-wqc85-8nxwc-gfd5v", // 41 characters
			template:    ptr.To[string]("{{ if le (len .machine.name) 20 }}{{ .machine.name }}{{else}}{{ trimSuffix \"-\" (trunc 14 .machine.name) }}-{{ trunc -5 .machine.name }}{{end}}"),
			want: []gomegatypes.GomegaMatcher{
				Equal("quick-start-d3-gfd5v"), // 20 characters
			},
		},
		{
			name:        "template for 20 characters: trim to 19 characters if name has more than 20 characters and last character of prefix is -",
			machineName: "quick-start-d-34gt4-md-0-wqc85-8nxwc-gfd5v", // 42 characters
			template:    ptr.To[string]("{{ if le (len .machine.name) 20 }}{{ .machine.name }}{{else}}{{ trimSuffix \"-\" (trunc 14 .machine.name) }}-{{ trunc -5 .machine.name }}{{end}}"),
			want: []gomegatypes.GomegaMatcher{
				Equal("quick-start-d-gfd5v"), // 19 characters
			},
		},
		{
			name:        "template with a prefix and only 5 random character from the machine name",
			machineName: "quick-start-d-34gt4-md-0-wqc85-8nxwc-gfd5v", // 42 characters
			template:    ptr.To[string]("vm-{{ trunc -5 .machine.name }}"),
			want: []gomegatypes.GomegaMatcher{
				Equal("vm-gfd5v"), // 8 characters
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := GenerateMachineNameFromTemplate(tt.machineName, tt.template)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateMachineNameFromTemplate error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) > maxNameLength {
				t.Errorf("generated name should never be longer than %d, got %d", maxNameLength, len(got))
			}

			for _, matcher := range tt.want {
				g.Expect(got).To(matcher)
			}
		})
	}
}
