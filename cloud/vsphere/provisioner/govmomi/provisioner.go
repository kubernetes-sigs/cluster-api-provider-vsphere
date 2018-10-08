package govmomi

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	vpshereprovisionercommon "sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/provisioner/common"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
)

type Provisioner struct {
	clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface
	lister          v1alpha1.Interface
	eventRecorder   record.EventRecorder
	sessioncache    map[string]interface{}
	utils           *vpshereprovisionercommon.ProvisionerUtil
}

func New(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, k8sClient kubernetes.Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder) (*Provisioner, error) {
	return &Provisioner{
		clusterV1alpha1: clusterV1alpha1,
		lister:          lister,
		eventRecorder:   eventRecorder,
		sessioncache:    make(map[string]interface{}),
		utils:           vpshereprovisionercommon.New(clusterV1alpha1, k8sClient, lister, eventRecorder),
	}, nil
}
