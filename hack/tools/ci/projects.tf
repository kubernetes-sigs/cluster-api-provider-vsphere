
resource "vsphere_resource_pool" "cpi" {
  name                    = "cloud-provider-vsphere"
  parent_resource_pool_id = data.vsphere_compute_cluster.compute_cluster.resource_pool_id
}
resource "vsphere_resource_pool" "capi" {
  name                    = "cluster-api-provider-vsphere"
  parent_resource_pool_id = data.vsphere_compute_cluster.compute_cluster.resource_pool_id
}
resource "vsphere_resource_pool" "image-builder" {
  name                    = "image-builder"
  parent_resource_pool_id = data.vsphere_compute_cluster.compute_cluster.resource_pool_id
}

resource "vsphere_folder" "cpi" {
  path          = "cloud-provider-vsphere"
  type          = "vm"
  datacenter_id = data.vsphere_datacenter.datacenter.id
}
resource "vsphere_folder" "capi" {
  path          = "cluster-api-provider-vsphere"
  type          = "vm"
  datacenter_id = data.vsphere_datacenter.datacenter.id
}
resource "vsphere_folder" "image-builder" {
  path          = "image-builder"
  type          = "vm"
  datacenter_id = data.vsphere_datacenter.datacenter.id
}

