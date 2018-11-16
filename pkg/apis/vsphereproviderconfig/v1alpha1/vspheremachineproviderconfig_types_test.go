/*
Copyright 2018 The Kubernetes Authors.

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

package v1alpha1

import (
	"testing"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestStorageVsphereMachineProviderConfig(t *testing.T) {
	key := types.NamespacedName{
		Name:      "foo",
		Namespace: "default",
	}
	created := &VsphereMachineProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		VsphereMachine: "machine",
		MachineRef:     "ref123",
		MachineSpec: VsphereMachineSpec{
			Datacenter:   "dc1",
			Datastore:    "ds1",
			ResourcePool: "rp1",
			Networks: []NetworkSpec{
				NetworkSpec{
					NetworkName: "net1",
					IPConfig: IPConfig{
						NetworkType: "dhcp",
						IP:          "1.2.3.4",
						Netmask:     "255.255.255.0",
						Gateway:     "1.2.3.1",
						Dns:         []string{"1.2.3.10"},
					},
				}},
			NumCPUs:    10,
			MemoryMB:   1000,
			VMTemplate: "mytemplate",
			Disks: []DiskSpec{
				DiskSpec{
					DiskSizeGB: 1,
					DiskLabel:  "disk0",
				},
			},
			Preloaded:        false,
			VsphereCloudInit: false,
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &VsphereMachineProviderConfig{}
	g.Expect(c.Create(context.TODO(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(context.TODO(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(context.TODO(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(context.TODO(), key, fetched)).To(gomega.HaveOccurred())
}
