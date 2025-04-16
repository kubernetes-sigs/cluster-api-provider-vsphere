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

package vmware

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
)

func TestVSphereMachineTemplate_Validate(t *testing.T) {
	tests := []struct {
		name                          string
		namingStrategy                *vmwarev1.VirtualMachineNamingStrategy
		affinity                      *vmwarev1.VSphereMachineAffinity
		workerAntiAffinityFeatureGate bool
		wantErr                       bool
	}{
		{
			name:           "Should succeed if namingStrategy not set",
			namingStrategy: nil,
			wantErr:        false,
		},
		{
			name: "Should succeed if namingStrategy.template not set",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: nil,
			},
			wantErr: false,
		},
		{
			name: "Should succeed if namingStrategy.template is set to the fallback value",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("{{ .machine.name }}"),
			},
			wantErr: false,
		},
		{
			name: "Should succeed if namingStrategy.template is set to the Windows example",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("{{ if le (len .machine.name) 20 }}{{ .machine.name }}{{else}}{{ trimSuffix \"-\" (trunc 14 .machine.name) }}-{{ trunc -5 .machine.name }}{{end}}"),
			},
			wantErr: false,
		},
		{
			name: "Should fail if namingStrategy.template is set to an invalid template",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("{{ invalid"),
			},
			wantErr: true,
		},
		{
			name: "Should fail if namingStrategy.template is set to a valid template that renders an invalid name",
			namingStrategy: &vmwarev1.VirtualMachineNamingStrategy{
				Template: ptr.To[string]("-{{ .machine.name }}"), // Leading - is not valid for names.
			},
			wantErr: true,
		},
		{
			name: "Should fail if anti-affinity sets machinedeployment label while feature gate is disabled",
			affinity: &vmwarev1.VSphereMachineAffinity{
				MachineDeploymentMachineAntiAffinity: &vmwarev1.VSphereMachineMachineDeploymentMachineAntiAffinity{
					PreferredDuringSchedulingPreferredDuringExecution: []vmwarev1.TopologyMachineDeploymentAffinityTerm{{
						MatchLabelKeys: []string{
							"cluster.x-k8s.io/cluster-name",
							"cluster.x-k8s.io/deployment-name",
						},
						TopologyKey: vmwarev1.TopologyKeyESXIHost,
					}},
				},
			},
			wantErr: true,
		},
		{
			name: "Should not fail if anti-affinity sets machinedeployment label while feature gate is enabled",
			affinity: &vmwarev1.VSphereMachineAffinity{
				MachineDeploymentMachineAntiAffinity: &vmwarev1.VSphereMachineMachineDeploymentMachineAntiAffinity{
					PreferredDuringSchedulingPreferredDuringExecution: []vmwarev1.TopologyMachineDeploymentAffinityTerm{{
						MatchLabelKeys: []string{
							"cluster.x-k8s.io/cluster-name",
							"cluster.x-k8s.io/deployment-name",
						},
						TopologyKey: vmwarev1.TopologyKeyESXIHost,
					}},
				},
			},
			workerAntiAffinityFeatureGate: true,
			wantErr:                       false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			utilfeature.SetFeatureGateDuringTest(t, feature.Gates, feature.WorkerAntiAffinity, tc.workerAntiAffinityFeatureGate)

			vSphereMachineTemplate := &vmwarev1.VSphereMachineTemplate{
				Spec: vmwarev1.VSphereMachineTemplateSpec{
					Template: vmwarev1.VSphereMachineTemplateResource{
						Spec: vmwarev1.VSphereMachineSpec{
							NamingStrategy: tc.namingStrategy,
							Affinity:       tc.affinity,
						},
					},
				},
			}

			webhook := &VSphereMachineTemplateWebhook{}
			_, err := webhook.validate(context.Background(), nil, vSphereMachineTemplate)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
