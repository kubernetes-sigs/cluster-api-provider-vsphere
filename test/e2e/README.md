# Testing

This document is to help developers understand how to test CAPV.

## e2e

This section illustrates how to do end-to-end (e2e) testing with CAPV.

### Requirements

In order to run the e2e tests the following requirements must be met:

* Administrative access to a vSphere server
* The testing must occur on a host that can access the VMs deployed to vSphere via the network
* Ginkgo ([download](https://onsi.github.io/ginkgo/#getting-ginkgo))
* Docker ([download](https://www.docker.com/get-started))
* Kind v0.7.0+ ([download](https://kind.sigs.k8s.io))

### Environment variables

The first step to running the e2e tests is setting up the required environment variables:

| Environment variable       | Description                                                                                           | Example                                                                          |
| -------------------------- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `VSPHERE_SERVER`              | The IP address or FQDN of a vCenter 6.7u3 server                                                      | `my.vcenter.com`                                                                 |
| `VSPHERE_USERNAME`            | The username used to access the vSphere server                                                        | `my-username`                                                                    |
| `VSPHERE_PASSWORD`            | The password used to access the vSphere server                                                        | `my-password`                                                                    |
| `VSPHERE_DATACENTER`          | The unique name or inventory path of the datacenter in which VMs will be created                      | `my-datacenter` or `/my-datacenter`                                              |
| `VSPHERE_FOLDER`              | The unique name or inventory path of the folder in which VMs will be created                          | `my-folder` or `/my-datacenter/vm/my-folder`                                     |
| `VSPHERE_RESOURCE_POOL`       | The unique name or inventory path of the resource pool in which VMs will be created                   | `my-resource-pool` or `/my-datacenter/host/Cluster-1/Resources/my-resource-pool` |
| `VSPHERE_DATASTORE`           | The unique name or inventory path of the datastore in which VMs will be created                       | `my-datastore` or `/my-datacenter/datstore/my-datastore`                         |
| `VSPHERE_DATASTORE_CLUSTER`   | The unique name or inventory path of the datastore cluster in which VMs will be created
| `VSPHERE_NETWORK`             | The unique name or inventory path of the network to which VMs will be connected                       | `my-network` or `/my-datacenter/network/my-network`                              |
| `VSPHERE_HAPROXY_TEMPLATE`    | The unique name or inventory path of the template from which the HAProxy load balancer VMs are cloned | `my-haproxy-template` or `/my-datacenter/vm/my-haproxy-template`                 |
| `VSPHERE_SSH_PRIVATE_KEY`     | The file path of the private key used to ssh into the CAPV VMs                                        | `/home/foo/bar-ssh.key`                                                          |
| `VSPHERE_SSH_AUTHORIZED_KEY`  | The public key that is added to the CAPV VMs                                                          | `ss-rsa ABCDEF...XYZ=`                                                          |

### Running the e2e tests

Run the following command to execute the CAPV e2e tests:

```shell
make e2e GC_KIND=false
```

The above command should build the CAPV manager image locally and use that image with the e2e test suite.
