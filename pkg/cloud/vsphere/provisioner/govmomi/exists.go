package govmomi

import (
	"context"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (pv *Provisioner) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		klog.V(4).Infof("Exists check, session from provider config error: %s", err.Error())
		return false, err
	}
	existsctx, cancel := context.WithCancel(*s.context)
	defer cancel()

	moref, err := vsphereutils.GetMachineRef(machine)
	if err != nil {
		klog.V(4).Infof("Exists check, GetMachineRef failed: %s", err.Error())
		return false, err
	}

	if moref == "" {
		return false, nil
	}

	var vm mo.VirtualMachine
	vmref := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: moref,
	}
	err = s.session.RetrieveOne(existsctx, vmref, []string{"name"}, &vm)
	if err != nil {
		klog.V(4).Infof("Exists check, RetrieveOne failed: %s", err.Error())
		return false, nil
	}
	klog.V(4).Infof("Exists check, found [%s, %s]", cluster.Name, machine.Name)

	return true, nil
}
