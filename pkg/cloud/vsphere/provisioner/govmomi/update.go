package govmomi

import (
	"context"
	"errors"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (pv *Provisioner) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if cluster == nil {
		return errors.New(constants.ClusterIsNullErr)
	}

	// Fetch any active task in vsphere if any
	// If an active task is there,

	klog.V(4).Infof("govmomi.Actuator.Update %s", machine.Spec.Name)

	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return err
	}
	updatectx, cancel := context.WithCancel(*s.context)
	defer cancel()

	moref, err := vsphereutils.GetMachineRef(machine)
	if err != nil {
		return err
	}
	var vmmo mo.VirtualMachine
	vmref := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: moref,
	}
	err = s.session.RetrieveOne(updatectx, vmref, []string{"name", "runtime"}, &vmmo)
	if err != nil {
		return nil
	}
	if vmmo.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOn {
		klog.Warningf("Machine %s is not running, rather it is in %s state", vmmo.Name, vmmo.Runtime.PowerState)
		return fmt.Errorf("Machine %s is not running, rather it is in %s state", vmmo.Name, vmmo.Runtime.PowerState)
	}

	if _, err := vsphereutils.GetIP(cluster, machine); err != nil {
		klog.V(4).Info("actuator.Update() - did not find IP, waiting on IP")
		vm := object.NewVirtualMachine(s.session.Client, vmref)
		vmIP, err := vm.WaitForIP(updatectx)
		if err != nil {
			return err
		}
		pv.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "IP Detected", "IP %s detected for Virtual Machine %s", vmIP, vm.Name())
		return pv.updateIP(cluster, machine, vmIP)
	}
	return nil
}

// Updates the detected IP for the machine and updates the cluster object signifying a change in the infrastructure
func (pv *Provisioner) updateIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine, vmIP string) error {
	klog.V(4).Infof(
		"updating machine IP address %s=%s %s=%s %s=%s %s=%s %s=%s",
		"cluster-namespace", cluster.Namespace,
		"cluster-name", cluster.Name,
		"machine-namespace", machine.Namespace,
		"machine-name", machine.Name,
		"ip-addr", vmIP)
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}
	machine.Annotations[constants.VmIpAnnotationKey] = vmIP
	return nil
}
