package govmomi

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Delete the machine
func (pv *Provisioner) Delete(ctx context.Context, cluster *clusterv1.Cluster, _machine *clusterv1.Machine) error {
	gomachine, err := pv.NewGovmomiMachine(cluster, _machine)
	if err != nil {
		return err
	}

	if on, err := gomachine.IsPoweredOn(ctx); on {
		if err != nil {
			return err
		}
		// TODO(moshloop): Don't wait, requeue and reconcile on the next loop
		gomachine.Eventf("PoweringOff", "Powering off machine")
		if err := gomachine.PowerOff(ctx).WaitFor(ctx); err != nil {
			return err
		}
	}
	gomachine.Eventf("Killing", "Killing machine %v", gomachine.Name)
	if err = gomachine.Delete(ctx).WaitFor(); err == nil {
		gomachine.Eventf("Killed", "Machine deletion complete")
	} else if err.Error() == "ServerFaultCode: The object 'vim.VirtualMachine:' has already been deleted or has not been completely created" {
		gomachine.Eventf("Killed", fmt.Sprintf("Machine deletion ignored, %s", err))
		return nil
	} else {
		gomachine.Eventf("Killed", fmt.Sprintf("Machine deletion failed %s", err))
		return err
	}
	return nil
}
