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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/namedmachines"
	"sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/provisioner/terraform"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/cloud/vsphere/vsphereproviderconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	v1alpha1 "sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions/cluster/v1alpha1"
)

type VsphereClient struct {
	clusterV1alpha1   clusterv1alpha1.ClusterV1alpha1Interface
	scheme            *runtime.Scheme
	codecFactory      *serializer.CodecFactory
	lister            v1alpha1.Interface
	namedMachineWatch *namedmachines.ConfigWatch
	eventRecorder     record.EventRecorder
	// Once the vsphere-deployer is deleted, both DeploymentClient and VsphereClient can depend on
	// something that implements GetIP instead of the VsphereClient depending on DeploymentClient.
	deploymentClient *DeploymentClient

	terraformProvisioner *terraform.Provisioner
}

func NewMachineActuator(clusterV1alpha1 clusterv1alpha1.ClusterV1alpha1Interface, lister v1alpha1.Interface, eventRecorder record.EventRecorder, namedMachinePath string) (*VsphereClient, error) {
	scheme, codecFactory, err := vsphereconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	deploymentClient := NewDeploymentClient()
	provisioner, err := terraform.New(clusterV1alpha1, lister, eventRecorder, namedMachinePath, deploymentClient)
	if err != nil {
		return nil, err
	}

	return &VsphereClient{
		clusterV1alpha1:      clusterV1alpha1,
		scheme:               scheme,
		codecFactory:         codecFactory,
		lister:               lister,
		eventRecorder:        eventRecorder,
		deploymentClient:     deploymentClient,
		terraformProvisioner: provisioner,
	}, nil
}

func (vc *VsphereClient) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	//creator := nativeprovisioner.NewCreator()
	//creator.Create(cluster, machine)

	if vc.terraformProvisioner != nil {
		return vc.terraformProvisioner.Create(cluster, machine)
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Delete(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	//deleter := nativeprovisioner.NewDeleter()
	//deleter.Delete(cluster, machine)

	if vc.terraformProvisioner != nil {
		return vc.terraformProvisioner.Delete(cluster, machine)
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Update(cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	//updater := nativeprovisioner.NewUpdater()
	//updater.Update(cluster, goalMachine)

	if vc.terraformProvisioner != nil {
		return vc.terraformProvisioner.Update(cluster, goalMachine)
	}

	return fmt.Errorf("No provisioner available")
}

func (vc *VsphereClient) Exists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	//validator := nativeprovisioner.NewValidator()
	//validator.Exists(cluster, goalMachine)

	if vc.terraformProvisioner != nil {
		return vc.terraformProvisioner.Exists(cluster, machine)
	}

	return false, fmt.Errorf("No provisioner available")
}
