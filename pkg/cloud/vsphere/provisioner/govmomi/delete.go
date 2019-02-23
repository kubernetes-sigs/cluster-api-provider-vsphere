package govmomi

import (
	"context"
	"errors"

	"github.com/vmware/govmomi/object"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Delete the machine
func (pv *Provisioner) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if cluster == nil {
		return errors.New(constants.ClusterIsNullErr)
	}

	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return err
	}
	deletectx, cancel := context.WithCancel(*s.context)
	defer cancel()

	if exists, _ := pv.Exists(ctx, cluster, machine); exists {
		moref, err := vsphereutils.GetMachineRef(machine)
		if err != nil {
			return err
		}
		var vm mo.VirtualMachine
		vmref := types.ManagedObjectReference{
			Type:  "VirtualMachine",
			Value: moref,
		}
		err = s.session.RetrieveOne(deletectx, vmref, []string{"name", "runtime.powerState"}, &vm)
		if err != nil {
			return err
		}
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Killing", "Killing machine %v", machine.Name)
		vmo := object.NewVirtualMachine(s.session.Client, vmref)
		if vm.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
			task, err := vmo.PowerOff(deletectx)
			if err != nil {
				klog.Infof("Error trigerring power off operation on the Virtual Machine %s", vm.Name)
				return err
			}
			err = task.Wait(deletectx)
			if err != nil {
				klog.Infof("Error powering off the Virtual Machine %s", vm.Name)
				return err
			}
		}
		task, err := vmo.Destroy(deletectx)
		taskinfo, err := task.WaitForResult(deletectx, nil)
		if taskinfo.State == types.TaskInfoStateSuccess {
			klog.Infof("Virtual Machine %v deleted successfully", vm.Name)
			pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Killed", "Machine %v deletion complete", machine.Name)
			return nil
		}
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Killed", "Machine %v deletion complete", machine.Name)
		klog.Errorf("VM Deletion failed on pv with following reason %v", taskinfo.Reason)
		return errors.New("VM Deletion failed")
	}
	return nil
}
