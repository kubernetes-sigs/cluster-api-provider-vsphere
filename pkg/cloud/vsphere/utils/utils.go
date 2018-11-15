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
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/labels"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/util"
	"sigs.k8s.io/yaml"
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

func GetMachineProviderStatus(machine *clusterv1.Machine) (*vsphereconfigv1.VsphereMachineProviderStatus, error) {
	if machine.Status.ProviderStatus == nil {
		return nil, nil
	}
	status := &vsphereconfigv1.VsphereMachineProviderStatus{}
	err := json.Unmarshal(machine.Status.ProviderStatus.Raw, status)
	if err != nil {
		glog.V(4).Infof("error unmarshaling machine provider status: %s", err.Error())
		return nil, err
	}
	return status, nil
}

func GetClusterProviderStatus(cluster *clusterv1.Cluster) (*vsphereconfigv1.VsphereClusterProviderStatus, error) {
	if cluster.Status.ProviderStatus == nil {
		return nil, nil
	}
	status := &vsphereconfigv1.VsphereClusterProviderStatus{}
	err := json.Unmarshal(cluster.Status.ProviderStatus.Raw, status)
	if err != nil {
		glog.V(4).Infof("error unmarshaling cluster provider status: %s", err.Error())

		return nil, err
	}
	return status, nil
}

func GetMachineProviderConfig(providerConfig clusterv1.ProviderConfig) (*vsphereconfigv1.VsphereMachineProviderConfig, error) {
	config := &vsphereconfigv1.VsphereMachineProviderConfig{}

	err := yaml.Unmarshal(providerConfig.Value.Raw, config)
	if err != nil {
		return nil, fmt.Errorf("machine providerconfig unmarshalling failure: %s", err.Error())
	}
	return config, nil
}

func GetClusterProviderConfig(providerConfig clusterv1.ProviderConfig) (*vsphereconfigv1.VsphereClusterProviderConfig, error) {
	config := &vsphereconfigv1.VsphereClusterProviderConfig{}

	err := yaml.Unmarshal(providerConfig.Value.Raw, config)
	if err != nil {
		return nil, fmt.Errorf("cluster providerconfig unmarshalling failure: %s", err.Error())
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
	pc, err := GetMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return "", err
	}
	return pc.MachineRef, nil
}

func GetActiveTasks(machine *clusterv1.Machine) string {
	ps, err := GetMachineProviderStatus(machine)
	if err != nil || ps == nil {
		return ""
	}
	return ps.TaskRef
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
				glog.Warningf("Error removing file '%s': %v", path, err)
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
		glog.Info("cannot get kubeconfig because found no IP")
		return "", err
	}
	glog.Infof("pulling kubeconfig (using ssh) from %s", ip)
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
	if err != nil {
		glog.Infof("ssh failed with error = %s", err.Error())
	}
	result := strings.TrimSpace(out.String())
	if len(result) > 0 {
		glog.Info("ssh pulled kubeconfig")
	}
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

func IsValidUUID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}

func GetNextBackOff() time.Duration {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = constants.RequeueAfterSeconds
	b.MaxInterval = constants.RequeueAfterSeconds + 10*time.Second
	b.Reset()
	return b.NextBackOff()
}
