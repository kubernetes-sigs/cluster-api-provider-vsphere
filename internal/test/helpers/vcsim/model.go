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

import "fmt"

const (
	// DefaultNetworkName is the name of the default network that exists when starting a new vcsim instance.
	DefaultNetworkName = "VM Network"

	// DefaultStoragePolicyName is the name of the default storage policy that exists when starting a new vcsim instance.
	DefaultStoragePolicyName = "vSAN Default Storage Policy"

	// DefaultVMTemplateName is the name of the default VM template the vcsim controller adds to new vcsim instance.
	// Note: There are no default templates when starting a new vcsim instance.
	// Note: For the sake of testing with vcsim the template doesn't really matter (nor the version of K8s hosted on it)
	// so the vcsim controller creates only a VM template with a well-known name.
	DefaultVMTemplateName = "ubuntu-2204-kube-vX"
)

// DatacenterName provide a function to compute vcsim datacenter names given its index.
func DatacenterName(datacenter int) string {
	return fmt.Sprintf("DC%d", datacenter)
}

// ClusterName provide a function to compute vcsim cluster names given its index and the index of a datacenter.
func ClusterName(datacenter, cluster int) string {
	return fmt.Sprintf("%s_C%d", DatacenterName(datacenter), cluster)
}

// ClusterPath provides the path for a vcsim cluster given its index and the index of a datacenter.
func ClusterPath(datacenter, cluster int) string {
	return fmt.Sprintf("/%s/host/%s", DatacenterName(datacenter), ClusterName(datacenter, cluster))
}

// DatastoreName provide a function to compute vcsim datastore names given its index.
func DatastoreName(datastore int) string {
	return fmt.Sprintf("LocalDS_%d", datastore)
}

// DatastorePath provides the path for a vcsim datastore given its index and the index of a datacenter.
func DatastorePath(datacenter, datastore int) string {
	return fmt.Sprintf("/%s/datastore/%s", DatacenterName(datacenter), DatastoreName(datastore))
}

// ResourcePoolPath provides the path for a vcsim Resources folder given the index of a datacenter and the index of a cluster.
func ResourcePoolPath(datacenter, cluster int) string {
	return fmt.Sprintf("/%s/host/%s/Resources", DatacenterName(datacenter), ClusterName(datacenter, cluster))
}

// VMFolderName provide a function to compute vcsim vm folder name names given the index of a datacenter.
func VMFolderName(datacenter int) string {
	return fmt.Sprintf("%s/vm", DatacenterName(datacenter))
}

// VMPath provides the path for a vcsim VM given the index of a datacenter and the vm name.
func VMPath(datacenter int, vm string) string {
	return fmt.Sprintf("/%s/%s", VMFolderName(datacenter), vm)
}

// NetworkFolderName provide a function to compute vcsim network folder name names given the index of a datacenter.
func NetworkFolderName(datacenter int) string {
	return fmt.Sprintf("%s/network", DatacenterName(datacenter))
}

// NetworkPath provides the path for a vcsim network given the index of a datacenter and the network name.
func NetworkPath(datacenter int, network string) string {
	return fmt.Sprintf("/%s/%s", NetworkFolderName(datacenter), network)
}
