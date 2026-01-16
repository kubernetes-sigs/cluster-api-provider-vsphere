/*
Copyright 2026 The Kubernetes Authors.

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

package client

import (
	"testing"

	. "github.com/onsi/gomega"
	vmoprv1alpha2 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
)

func Test_WatchObject(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		obj           client.Object
		wantObj       client.Object
		wantErr       bool
	}{
		{
			name:          "Create watch object for VirtualMachine when target version is v1alpha2",
			targetVersion: vmoprv1alpha2.GroupVersion.Version,
			obj:           &vmoprvhub.VirtualMachine{},
			wantObj:       &vmoprv1alpha2.VirtualMachine{},
		},
		{
			name:          "Create watch object for VirtualMachine when target version is v1alpha5",
			targetVersion: vmoprv1alpha5.GroupVersion.Version,
			obj:           &vmoprvhub.VirtualMachine{},
			wantObj:       &vmoprv1alpha5.VirtualMachine{},
		},
		{
			name:    "Fails for non hub objects",
			obj:     &corev1.Node{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			converter.SetTargetVersion(tt.targetVersion)

			c := &conversionClient{
				internalClient: fake.NewClientBuilder().WithScheme(scheme).Build(),
				converter:      converter,
			}

			gotObj, err := WatchObject(c, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(gotObj).To(Equal(tt.wantObj))
		})
	}
}
