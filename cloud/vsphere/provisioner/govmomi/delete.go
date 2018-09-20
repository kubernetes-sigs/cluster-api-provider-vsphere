package govmomi

import (
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type Deleter struct {
}

func NewDeleter() *Deleter {
	return &Deleter{}
}

// Create the machine.
func (c *Deleter) Deleter(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	return nil
}
