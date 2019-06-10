package govmomi

import (
	"context"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

func (pv *Provisioner) Update(ctx context.Context, cluster *clusterv1.Cluster, _machine *clusterv1.Machine) error {
	machine, err := pv.NewGovmomiMachine(cluster, _machine)
	if err != nil {
		return err
	}

	ip, err := machine.GetIP()

	if err != nil {
		// TODO(moshloop): don't wait for the IP, requeue and check on the next reconcile loop
		ip, err = machine.WaitForIP(ctx)
		if err != nil {
			return err
		}
	}
	machine.Eventf("IP Detected", "%s", ip)
	return machine.UpdateAnnotations(map[string]string{constants.VmIpAnnotationKey: ip})
}
