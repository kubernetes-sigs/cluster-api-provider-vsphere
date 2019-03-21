# Creating a Custom Cloud Init Image with the Guestinfo Datasource

The cluster api vSphere provider allows machines to use either DHCP or static IP.  The provider also
currently relies on a cloud init image to bootstrap the VM as a k8s node.  If a public cloud init
image (e.g. one downloaded from Ubuntu) is used, a machine can only use DHCP.  To create a machine that
uses static IP, a custom cloud init image that has a [GuestInfo Datasource](https://github.com/vmware/cloud-init-vmware-guestinfo)
install must be used.

Creating this custom cloud init image can be created manually.  The steps are as follows,

1. Download a public cloud init image
2. Create a VM with that image
3. Install the [Guestinfo Datasource](https://github.com/vmware/cloud-init-vmware-guestinfo)
    - Installation is all that is needed.  The configuration steps on the projects' repo page can be ignored.
4. Power down the VM
5. Convert the VM into a VM template through the vSphere client (e.g. [instructions](https://docs.vmware.com/en/VMware-vSphere/6.7/com.vmware.vsphere.vm_admin.doc/GUID-FE6DE4DF-FAD0-4BB0-A1FD-AFE9A40F4BFE.html)).

