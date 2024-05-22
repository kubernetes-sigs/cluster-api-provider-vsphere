variable "vsphere_user" {
  type    = string
  default = "administrator@vsphere.local"
}

variable "vsphere_password" {
  type    = string
}

variable "vsphere_server" {
  type    = string
}

variable "nsxt_user" {
  type    = string
  default = "admin"
}

variable "nsxt_password" {
  type    = string
}

variable "nsxt_server" {
  type    = string
}

variable "vsphere_datacenter" {
  type    = string
}

variable "vsphere_cluster" {
  type    = string
}

variable "vsphere_datastorename" {
  type    = string
}
