/*
Copyright 2023 The Kubernetes Authors.

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

package kubevip

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/kubevip"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
)

// TopologyVariable returns the ClusterClass variable for kube-vip.
func TopologyVariable() (*clusterv1.ClusterVariable, error) {
	out, err := json.Marshal(kubevip.PodYAML())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to json-encode variable kubeVipPod")
	}

	return &clusterv1.ClusterVariable{
		Name: "kubeVipPodManifest",
		Value: apiextensionsv1.JSON{
			Raw: out,
		},
	}, nil
}

// TopologyPatch returns the ClusterClass patch for kube-vip.
func TopologyPatch() clusterv1.ClusterClassPatch {
	patches := []clusterv1.JSONPatch{}

	for _, f := range kubevip.Files() {
		p := clusterv1.JSONPatch{
			Op:        "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/files/-",
			ValueFrom: &clusterv1.JSONPatchValue{},
		}

		// Special handling to make this patch work
		if f.Path == "/etc/kubernetes/manifests/kube-vip.yaml" {
			lines := []string{
				fmt.Sprintf("owner: %q", f.Owner),
				fmt.Sprintf("path: %q", f.Path),
				`content: {{ printf "%q" (regexReplaceAll "(name: address\n +value:).*" .kubeVipPodManifest (printf "$1 %s" .controlPlaneIpAddr)) }}`,
				fmt.Sprintf("permissions: %q", f.Permissions),
			}
			p.ValueFrom.Template = strings.Join(lines, "\n")
			patches = append(patches, p)
			continue
		}

		tpl, _ := fileToTemplate(f)
		p.ValueFrom.Template = tpl
		patches = append(patches, p)
	}

	// This two patches is part of the workaround for https://github.com/kube-vip/kube-vip/issues/684
	return clusterv1.ClusterClassPatch{
		Name: "kubeVipPodManifest",
		Definitions: []clusterv1.PatchDefinition{
			{
				Selector: clusterv1.PatchSelector{
					APIVersion: controlplanev1.GroupVersion.String(),
					Kind:       util.TypeToKind(&controlplanev1.KubeadmControlPlaneTemplate{}),
					MatchResources: clusterv1.PatchSelectorMatch{
						ControlPlane: ptr.To(true),
					},
				},
				JSONPatches: patches,
			},
		},
	}
}

func fileToTemplate(f bootstrapv1.File) (string, error) {
	out, err := yaml.Marshal(f)
	if err != nil {
		return "", errors.Wrapf(err, "unable to wrap file %q", f.Path)
	}

	return string(out), nil
}
