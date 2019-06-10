package govmomi

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (pv *Provisioner) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	gomachine, err := pv.NewGovmomiMachine(cluster, machine)
	if err != nil {
		return false, err
	}
	return gomachine.config.MachineRef != "", nil
}
