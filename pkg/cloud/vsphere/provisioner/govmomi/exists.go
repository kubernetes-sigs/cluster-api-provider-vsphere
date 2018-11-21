package govmomi

import (
	"context"

	"github.com/golang/glog"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (pv *Provisioner) Exists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		glog.V(4).Infof("Exists check, session from provider config error: %s", err.Error())
		return false, err
	}
	ctx, cancel := context.WithCancel(*s.context)
	defer cancel()

	moref, err := vsphereutils.GetVMId(machine)
	if err != nil {
		glog.V(4).Infof("Exists check, GetVMId failed: %s", err.Error())
		return false, err
	}
	var vm mo.VirtualMachine
	vmref := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: moref,
	}
	err = s.session.RetrieveOne(ctx, vmref, []string{"name"}, &vm)
	if err != nil {
		glog.V(4).Infof("Exists check, RetrieveOne failed: %s", err.Error())
		return false, nil
	}
	glog.V(4).Infof("Exists check, found [%s, %s]", cluster.Name, machine.Name)

	return true, nil
}
