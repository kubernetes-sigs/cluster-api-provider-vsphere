package govmomi

import (
	"errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
)

type Provisioner struct {
	clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface
	lister          v1alpha1.Interface
	eventRecorder   record.EventRecorder
	sessioncache    map[string]interface{}
	k8sClient       kubernetes.Interface
}

func New(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, k8sClient kubernetes.Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder) (*Provisioner, error) {
	return &Provisioner{
		clusterV1alpha1: clusterV1alpha1,
		lister:          lister,
		eventRecorder:   eventRecorder,
		sessioncache:    make(map[string]interface{}),
		k8sClient:       k8sClient,
	}, nil
}

func (pv *Provisioner) NewGovmomiMachine(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*GovmomiMachine, error) {
	if cluster == nil {
		return nil, errors.New(constants.ClusterIsNullErr)
	}

	s, err := pv.sessionFromProviderConfig(cluster, machine)
	if err != nil {
		return nil, err
	}

	gomachine, err := NewGovmomiMachine(cluster, machine, s, pv.eventRecorder, pv.k8sClient, pv.clusterV1alpha1)
	if err != nil {
		return nil, err
	}

	return gomachine, nil
}
