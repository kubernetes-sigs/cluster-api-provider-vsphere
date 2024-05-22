data "nsxt_policy_tier1_gateway" "tier1" {
  display_name = "tier1"
}

resource "nsxt_policy_dhcp_server" "upstream-vsphere" {
  display_name      = "upstream-vsphere"
  description       = "Terraform provisioned DhcpServerConfig"
  lease_time        = 600
  server_addresses  = ["192.168.6.2/24"]
  edge_cluster_path = data.nsxt_policy_edge_cluster.edge-cluster-0.path
}

data "nsxt_policy_edge_cluster" "edge-cluster-0" {
  display_name = "edge-cluster-0"
}

# TODO: this still fails
resource "nsxt_policy_segment" "upstream-vsphere" {
  # context {
  #   project_id = data.nsxt_policy_project.default.id
  # }
  display_name      = "upstream-vsphere"
  description       = "Terraform provisioned Segment for upstream testing"
  transport_zone_path = data.nsxt_policy_tier1_gateway.tier1.path

  dhcp_config_path = nsxt_policy_dhcp_server.upstream-vsphere.path

  subnet {
    cidr        = "192.168.6.1/24"
    dhcp_ranges = ["192.168.6.3-192.168.6.160"]

    dhcp_v4_config {
      server_address = "192.168.6.2/24"
      lease_time     = 600
      # dhcp_option_121 {
      #   network  = "6.6.6.0/24"
      #   next_hop = "1.1.1.21"
      # }

      # dhcp_generic_option {
      #   code   = "119"
      #   values = ["abc"]
      # }
    }
  }
}
