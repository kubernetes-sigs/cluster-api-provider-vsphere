package govmomi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (pv *Provisioner) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	// Fetch any active task in vsphere if any
	// If an active task is there,

	klog.V(4).Infof("govmomi.Actuator.Update %s", machine.Spec.Name)

	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return err
	}
	updatectx, cancel := context.WithCancel(*s.context)
	defer cancel()

	moref, err := vsphereutils.GetVMId(machine)
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
	nmachine := machine.DeepCopy()
	if nmachine.ObjectMeta.Annotations == nil {
		nmachine.ObjectMeta.Annotations = make(map[string]string)
	}
	klog.V(4).Infof("updateIP - IP = %s", vmIP)
	nmachine.ObjectMeta.Annotations[constants.VmIpAnnotationKey] = vmIP
	_, err := pv.clusterV1alpha1.Machines(nmachine.Namespace).Update(nmachine)
	if err != nil {
		return err
	}
	// Update the cluster status with updated time stamp for tracking purposes
	status := &vsphereconfigv1.VsphereClusterProviderStatus{LastUpdated: time.Now().UTC().String()}
	out, err := json.Marshal(status)
	ncluster := cluster.DeepCopy()
	ncluster.Status.ProviderStatus = &runtime.RawExtension{Raw: out}
	_, err = pv.clusterV1alpha1.Clusters(ncluster.Namespace).UpdateStatus(ncluster)
	return err
}
