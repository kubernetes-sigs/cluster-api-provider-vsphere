package utils

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/constants"
	vsphereconfig "sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/vsphereproviderconfig"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/vsphereproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/util"
)

// GetMasterForCluster returns the master nodes for the given cluster
func GetMasterForCluster(cluster *clusterv1.Cluster, lister v1alpha1.Interface) ([]*clusterv1.Machine, error) {
	masters := make([]*clusterv1.Machine, 0)
	machines, err := lister.Machines().Lister().Machines(cluster.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		if util.IsMaster(machine) {
			masters = append(masters, machine)
			// Return the first master for now. Need to handle the multi-master case better
			glog.Infof("Found the master VM %s", machine.Name)
		}
	}
	if len(masters) == 0 {
		glog.Infof("No master node found for the cluster %s", cluster.Name)
	}
	return masters, nil
}

func GetIP(_ *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	if machine.ObjectMeta.Annotations == nil {
		return "", errors.New("could not get IP")
	}
	if ip, ok := machine.ObjectMeta.Annotations[constants.VmIpAnnotationKey]; ok {
		glog.Infof("Returning IP from machine annotation %s", ip)
		return ip, nil
	}
	return "", errors.New("could not get IP")
}

func GetMachineProviderStatus(machine *clusterv1.Machine) (*vsphereconfig.VsphereMachineProviderStatus, error) {
	if machine.Status.ProviderStatus == nil {
		return nil, nil
	}
	_, codecFactory, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	obj, gvk, err := codecFactory.UniversalDecoder().Decode(machine.Status.ProviderStatus.Raw, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("machine providerstatus decoding failure: %v", err)
	}
	status, ok := obj.(*vsphereconfig.VsphereMachineProviderStatus)
	if !ok {
		return nil, fmt.Errorf("machine providerstatus failure to cast to vsphere; type: %v", gvk)
	}
	return status, nil
}

func GetClusterProviderStatus(cluster *clusterv1.Cluster) (*vsphereconfig.VsphereClusterProviderStatus, error) {
	if cluster.Status.ProviderStatus == nil {
		return nil, nil
	}
	_, codecFactory, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	obj, gvk, err := codecFactory.UniversalDecoder().Decode(cluster.Status.ProviderStatus.Raw, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("cluster providerstatus decoding failure: %v", err)
	}
	status, ok := obj.(*vsphereconfig.VsphereClusterProviderStatus)
	if !ok {
		return nil, fmt.Errorf("cluster providerstatus failure to cast to vsphere; type: %v", gvk)
	}
	return status, nil
}

func GetMachineProviderConfig(providerConfig clusterv1.ProviderConfig) (*vsphereconfig.VsphereMachineProviderConfig, error) {
	_, codecFactory, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	obj, gvk, err := codecFactory.UniversalDecoder().Decode(providerConfig.Value.Raw, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("machine providerconfig decoding failure: %v", err)
	}

	config, ok := obj.(*vsphereconfig.VsphereMachineProviderConfig)
	if !ok {
		return nil, fmt.Errorf("machine providerconfig failure to cast to vsphere; type: %v", gvk)
	}

	return config, nil
}

func GetClusterProviderConfig(providerConfig clusterv1.ProviderConfig) (*vsphereconfig.VsphereClusterProviderConfig, error) {
	_, codecFactory, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	obj, gvk, err := codecFactory.UniversalDecoder().Decode(providerConfig.Value.Raw, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("cluster providerconfig decoding failure: %v", err)
	}

	config, ok := obj.(*vsphereconfig.VsphereClusterProviderConfig)
	if !ok {
		return nil, fmt.Errorf("cluster providerconfig failure to cast to vsphere; type: %v", gvk)
	}

	return config, nil
}
