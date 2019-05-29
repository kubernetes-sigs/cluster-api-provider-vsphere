package govmomi

import (
	"context"
	"errors"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if exists, _ := pv.Exists(ctx, cluster, machine); exists {
		if err := pv.powerOffAndDestroy(ctx, cluster, machine); err != nil {
			return err
		}
	}

	// Delete the node object bound to the Machine via the Status.NodeRef reference
	if machine.Status.NodeRef != nil {
		// Do a force delete (grace period set to 0) as the underlying VM is already deleted
		err := pv.k8sClient.Core().Nodes().Delete(machine.Status.NodeRef.Name, metav1.NewDeleteOptions(0))
		if err != nil {
			// Log the warning for unable to delete and that the user can delete the node manually. This error
			// should not hold the Machine delete operation since the underlying VM is already deleted at this point
			klog.Warningf("Could not remove the node %q bound to the machine automatically. Please manually remove the node if it is not already removed. Error encountered: %s", machine.Status.NodeRef.Name, err.Error())
		}
	}
	return nil
}

func (pv *Provisioner) powerOffAndDestroy(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return err
	}
	deletectx, cancel := context.WithCancel(*s.context)
	defer cancel()

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
	pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Killing", "Killing machine %q", machine.Name)
	vmo := object.NewVirtualMachine(s.session.Client, vmref)
	if vm.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
		task, err := vmo.PowerOff(deletectx)
		if err != nil {
			klog.Errorf("Error trigerring power off operation on the Virtual Machine %q", vm.Name)
			return err
		}
		err = task.Wait(deletectx)
		if err != nil {
			klog.Errorf("Error powering off the Virtual Machine %q", vm.Name)
			return err
		}
	}
	task, err := vmo.Destroy(deletectx)
	taskinfo, err := task.WaitForResult(deletectx, nil)
	if taskinfo.State == types.TaskInfoStateSuccess {
		klog.Infof("Virtual Machine %q deleted successfully", vm.Name)
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Killed", "Machine %q deletion complete", machine.Name)
		return nil
	}
	pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Kill Failed", "Machine %q deletion failed", machine.Name)
	klog.Errorf("VM Deletion failed with following reason %q", taskinfo.Reason)
	return fmt.Errorf("Virtual Machine %q deletion failed", machine.Name)
}
