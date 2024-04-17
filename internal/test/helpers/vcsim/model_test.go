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

package vcsim

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_vcsim_NamesAndPath(t *testing.T) {
	g := NewWithT(t)

	datacenter := 5
	cluster := 3
	datastore := 7
	distributedPortGroup := 4

	g.Expect(DatacenterName(datacenter)).To(Equal("DC5"))
	g.Expect(ClusterName(datacenter, cluster)).To(Equal("DC5_C3"))
	g.Expect(ClusterPath(datacenter, cluster)).To(Equal("/DC5/host/DC5_C3"))
	g.Expect(DatastoreName(datastore)).To(Equal("LocalDS_7"))
	g.Expect(DatastorePath(datacenter, datastore)).To(Equal("/DC5/datastore/LocalDS_7"))
	g.Expect(ResourcePoolPath(datacenter, cluster)).To(Equal("/DC5/host/DC5_C3/Resources"))
	g.Expect(VMFolderName(datacenter)).To(Equal("DC5/vm"))
	g.Expect(VMPath(datacenter, "my-mv")).To(Equal("/DC5/vm/my-mv"))
	g.Expect(NetworkFolderName(datacenter)).To(Equal("DC5/network"))
	g.Expect(NetworkPath(datacenter, "my-network")).To(Equal("/DC5/network/my-network"))
	g.Expect(DistributedPortGroupName(datacenter, distributedPortGroup)).To(Equal("DC5_DVPG4"))
}
