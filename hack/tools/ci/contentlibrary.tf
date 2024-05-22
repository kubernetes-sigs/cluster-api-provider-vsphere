resource "vsphere_content_library" "capv" {
  name            = "capv"
  description     = "Content Library for CAPV."
  storage_backing = [data.vsphere_datastore.datastore.id]
}


resource "terraform_data" "cl_item_ubuntu-v1-30-0" {
  triggers_replace = vsphere_content_library.capv.id

  provisioner "local-exec" {
    command = "if [[ \"$(govc library.info capv/ubuntu-2204-kube-v1.30.0 || true)\" == \"\" ]]; then govc library.import -pull ${vsphere_content_library.capv.name} https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/releases/download/templates%2Fv1.30.0/ubuntu-2204-kube-v1.30.0.ova; fi;"
    environment = {
      "GOVC_URL" = "${var.vsphere_user}:${var.vsphere_password}@${var.vsphere_server}"
      "GOVC_INSECURE" = "true"
    }
    interpreter = [ "/bin/bash", "-c" ]

  }
}
