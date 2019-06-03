# Provisioning on ESXi

This cluster api provider is capable of provisioning clusters on ESXi with some limitations.  It was written as a 
productivity tool for developers of this provider.  Some of the limitations are as follows,

1. The vSphere cloud provider is not available on ESXi.  All features provided by the cloud provider will be 
unavailable to clusters provisioned to ESXi.
2. VM cloning is available on vCenter but not on ESXi.  The VM creation process currently is unable to resize the 
machine's disk yet.  This may change in the future.

Provioning on ESXi is very similar to provisioning on vCenter.  Make sure to read the [main Quickstart intro](
./README.md) to learn the basics.  This page will discuss the differences, which are simple changes to the cluster 
and machine definition files.  The vSphere Cluster API provider will automatically detect whether the infrastructure 
is vCenter or ESXi; however, some of the changes to the definition files will ensure the cluster is properly 
functioning once it is provisioned.

### Cluster definition files

The cluster definition file remain largely the same as for deploying to vCenter.  The one thing to watch out for is 
to make sure the cidr ranges do not overlap with the subnet of your ESXi's network.  If the pod cidr ranges overlap, 
the CNI plugin may fail to deploy correctly.  As an example, suppose a developer deploys an ESXi host and uses the 
internal gateway server as the DNS server, make sure the pod CIDR does not overlap with the ESXi host's DHCP IP range.

### Machine definition files

The machine definition files remain largely the same.  Make sure to leave `datacenter` and `resourcePool` empty.  
These fields are irrelevant for ESXi.