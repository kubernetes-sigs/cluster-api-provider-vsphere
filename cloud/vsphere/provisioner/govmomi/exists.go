package govmomi

import (
	"context"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (vc *Provisioner) Exists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	s, err := vc.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()

	moref, err := vsphereutils.GetVMId(machine)
	if err != nil {
		return false, err
	}
	var vm mo.VirtualMachine
	vmref := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: moref,
	}
	err = s.session.RetrieveOne(ctx, vmref, []string{"name"}, &vm)
	if err != nil {
		return false, nil
	}
	return true, nil
}
