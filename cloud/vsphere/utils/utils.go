package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"strings"

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
		return ip, nil
	}
	return "", errors.New("could not get IP")
}

func GetMachineProviderStatus(machine *clusterv1.Machine) (*vsphereconfig.VsphereMachineProviderStatus, error) {
	if machine.Status.ProviderStatus == nil {
		return nil, nil
	}
	status := &vsphereconfig.VsphereMachineProviderStatus{}
	err := json.Unmarshal(machine.Status.ProviderStatus.Raw, status)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func GetClusterProviderStatus(cluster *clusterv1.Cluster) (*vsphereconfig.VsphereClusterProviderStatus, error) {
	if cluster.Status.ProviderStatus == nil {
		return nil, nil
	}
	status := &vsphereconfig.VsphereClusterProviderStatus{}
	err := json.Unmarshal(cluster.Status.ProviderStatus.Raw, status)
	if err != nil {
		return nil, err
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

// Just a temporary hack to grab a single range from the config.
func GetSubnet(netRange clusterv1.NetworkRanges) string {
	if len(netRange.CIDRBlocks) == 0 {
		return ""
	}
	return netRange.CIDRBlocks[0]
}

func GetVMId(machine *clusterv1.Machine) (string, error) {
	if machine.ObjectMeta.Annotations != nil {
		if vmid, ok := machine.ObjectMeta.Annotations[constants.VirtualMachineRef]; ok {
			return vmid, nil
		}
	}
	return "", nil
}

func GetActiveTasks(machine *clusterv1.Machine) string {
	if machine.ObjectMeta.Annotations != nil {
		if taskref, ok := machine.ObjectMeta.Annotations[constants.VirtualMachineTaskRef]; ok {
			return taskref
		}
	}
	return ""
}

func CreateTempFile(contents string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		glog.Warningf("Error creating temporary file")
		return "", err
	}
	// For any error in this method, clean up the temp file
	defer func(pErr *error, path string) {
		if *pErr != nil {
			if err := os.Remove(path); err != nil {
				glog.Warningf("Error removing file '%v': %v", err)
			}
		}
	}(&err, tmpFile.Name())

	if _, err = tmpFile.Write([]byte(contents)); err != nil {
		glog.Warningf("Error writing to temporary file '%s'", tmpFile.Name())
		return "", err
	}
	if err = tmpFile.Close(); err != nil {
		return "", err
	}
	if err = os.Chmod(tmpFile.Name(), 0644); err != nil {
		glog.Warningf("Error setting file permission to 0644 for the temporary file '%s'", tmpFile.Name())
		return "", err
	}
	return tmpFile.Name(), nil
}

func GetKubeConfig(cluster *clusterv1.Cluster, master *clusterv1.Machine) (string, error) {
	ip, err := GetIP(cluster, master)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd := exec.Command(
		"ssh", "-i", "~/.ssh/vsphere_tmp",
		"-q",
		"-o", "StrictHostKeyChecking no",
		"-o", "UserKnownHostsFile /dev/null",
		fmt.Sprintf("ubuntu@%s", ip),
		"sudo cat /etc/kubernetes/admin.conf")
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	result := strings.TrimSpace(out.String())
	return result, err
}

// ByteToGiB returns n/1024^3. The input must be an integer that can be
// appropriately divisible.
func ByteToGiB(n int64) int64 {
	return n / int64(math.Pow(1024, 3))
}

// GiBToByte returns n*1024^3.
func GiBToByte(n int64) int64 {
	return int64(n * int64(math.Pow(1024, 3)))
}
