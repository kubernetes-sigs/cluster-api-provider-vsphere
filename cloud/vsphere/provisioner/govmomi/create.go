package govmomi

import (
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type Creator struct {
}

func NewCreator() *Creator {
	return &Creator{}
}

// Create the machine.
func (c *Creator) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	return nil
}
