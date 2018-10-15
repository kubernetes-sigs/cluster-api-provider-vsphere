/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vsphere

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/provisioner/govmomi"
	"sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/provisioner/terraform"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/vsphereproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
)

type VsphereClient struct {
	clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface
	scheme          *runtime.Scheme
	provisioner     machine.Actuator
}

func NewGovmomiMachineActuator(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, k8sClient kubernetes.Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder) (*VsphereClient, error) {
	scheme, _, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	provisioner, err := govmomi.New(clusterV1alpha1, k8sClient, lister, eventRecorder)
	if err != nil {
		return nil, err
	}

	return &VsphereClient{
		clusterV1alpha1: clusterV1alpha1,
		scheme:          scheme,
		provisioner:     provisioner,
	}, nil
}

func NewTerraformMachineActuator(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder, namedMachinePath string) (*VsphereClient, error) {
	scheme, _, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	deploymentClient := NewDeploymentClient()
	provisioner, err := terraform.New(clusterV1alpha1, lister, eventRecorder, namedMachinePath, deploymentClient)
	if err != nil {
		return nil, err
	}

	return &VsphereClient{
		clusterV1alpha1: clusterV1alpha1,
		scheme:          scheme,
		provisioner:     provisioner,
	}, nil
}

func (vc *VsphereClient) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if vc.provisioner != nil {
		err := vc.provisioner.Create(cluster, machine)
		if err != nil {
			glog.Error(err)
			return err
		}
		return nil
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Delete(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	if vc.provisioner != nil {
		return vc.provisioner.Delete(cluster, machine)
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Update(cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	if vc.provisioner != nil {
		return vc.provisioner.Update(cluster, goalMachine)
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Exists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	if vc.provisioner != nil {
		return vc.provisioner.Exists(cluster, machine)
	}

	return false, fmt.Errorf("No provisioner available")
}
