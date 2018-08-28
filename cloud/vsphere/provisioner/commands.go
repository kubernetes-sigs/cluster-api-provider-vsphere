package provisioner

import (
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type ClusterCreator interface {
	// Create the machine.
	Create(*clusterv1.Cluster, *clusterv1.Machine) error
}

type ClusterDeleter interface {
	// Delete the machine. If no error is returned, it is assumed that all dependent resources have been cleaned up.
	Delete(*clusterv1.Cluster, *clusterv1.Machine) error
}

type ClusterUpdater interface {
	// Update the machine to the provided definition.
	Update(*clusterv1.Cluster, *clusterv1.Machine) error
}

type ClusterValidator interface {
	// Checks if the machine currently exists.
	Exists(*clusterv1.Cluster, *clusterv1.Machine) (bool, error)
}