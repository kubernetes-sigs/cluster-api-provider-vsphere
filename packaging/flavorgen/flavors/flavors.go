/*
Copyright 2020 The Kubernetes Authors.

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

// Package flavors contains tools to generate CAPV templates.
package flavors

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/crs"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/kubevip"
)

const (
	// Supported workload cluster flavors.

	VIP                       = "vip"
	ExternalLoadBalancer      = "external-loadbalancer"
	Ignition                  = "ignition"
	ClusterClass              = "cluster-class"
	ClusterTopology           = "cluster-topology"
	NodeIPAM                  = "node-ipam"
	Supervisor                = "supervisor"
	ClusterClassSupervisor    = "cluster-class-supervisor"
	ClusterTopologySupervisor = "cluster-topology-supervisor"
)

func ClusterClassTemplateWithKubeVIP() []runtime.Object {
	vSphereClusterTemplate := newVSphereClusterTemplate()
	clusterClass := newClusterClass()
	machineTemplate := newVSphereMachineTemplate(fmt.Sprintf("%s-template", env.ClusterClassNameVar))
	workerMachineTemplate := newVSphereMachineTemplate(fmt.Sprintf("%s-worker-machinetemplate", env.ClusterClassNameVar))
	controlPlaneTemplate := newKubeadmControlPlaneTemplate(fmt.Sprintf("%s-controlplane", env.ClusterClassNameVar))
	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s-worker-bootstrap-template", env.ClusterClassNameVar), false)

	ClusterClassTemplate := []runtime.Object{
		&vSphereClusterTemplate,
		&clusterClass,
		&machineTemplate,
		&workerMachineTemplate,
		&controlPlaneTemplate,
		&kubeadmJoinTemplate,
	}
	return ClusterClassTemplate
}

func ClusterClassTemplateSupervisor() []runtime.Object {
	vSphereClusterTemplate := newVMWareClusterTemplate()
	clusterClass := newVMWareClusterClass()
	machineTemplate := newVMWareMachineTemplate(fmt.Sprintf("%s-template", env.ClusterClassNameVar))
	workerMachineTemplate := newVMWareMachineTemplate(fmt.Sprintf("%s-worker-machinetemplate", env.ClusterClassNameVar))
	controlPlaneTemplate := newKubeadmControlPlaneTemplate(fmt.Sprintf("%s-controlplane", env.ClusterClassNameVar))
	controlPlaneTemplate.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands = append([]string{"dhclient eth0"}, controlPlaneTemplate.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands...)
	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s-worker-bootstrap-template", env.ClusterClassNameVar), false)
	kubeadmJoinTemplate.Spec.Template.Spec.PreKubeadmCommands = append([]string{"dhclient eth0"}, kubeadmJoinTemplate.Spec.Template.Spec.PreKubeadmCommands...)

	ClusterClassTemplate := []runtime.Object{
		&vSphereClusterTemplate,
		&clusterClass,
		&machineTemplate,
		&workerMachineTemplate,
		&controlPlaneTemplate,
		&kubeadmJoinTemplate,
	}
	return ClusterClassTemplate
}

func ClusterTopologyTemplateKubeVIP() ([]runtime.Object, error) {
	cluster, err := newClusterTopologyCluster()
	if err != nil {
		return nil, err
	}
	identitySecret := newIdentitySecret()
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI, err := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	if err != nil {
		return nil, err
	}
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&identitySecret,
		&clusterResourceSet,
	}
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)
	return MultiNodeTemplate, nil
}

func ClusterTopologyTemplateSupervisor() ([]runtime.Object, error) {
	cluster, err := newClusterTopologyCluster()
	if err != nil {
		return nil, err
	}
	identitySecret := newIdentitySecret()
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI, err := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	if err != nil {
		return nil, err
	}
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&identitySecret,
		&clusterResourceSet,
	}
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)
	return MultiNodeTemplate, nil
}

func MultiNodeTemplateWithKubeVIP() ([]runtime.Object, error) {
	vsphereCluster := newVSphereCluster()
	cpMachineTemplate := newVSphereMachineTemplate(env.ClusterNameVar)
	workerMachineTemplate := newVSphereMachineTemplate(fmt.Sprintf("%s-worker", env.ClusterNameVar))
	controlPlane := newKubeadmControlplane(&cpMachineTemplate, nil)
	kubevip.PatchControlPlane(&controlPlane)

	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s%s", env.ClusterNameVar, env.MachineDeploymentNameSuffix), true)
	cluster := newCluster(&vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, &workerMachineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI, err := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	if err != nil {
		return nil, err
	}
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	identitySecret := newIdentitySecret()

	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&vsphereCluster,
		&cpMachineTemplate,
		&workerMachineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
		&clusterResourceSet,
		&identitySecret,
	}

	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)

	return MultiNodeTemplate, nil
}

func MultiNodeTemplateSupervisor() ([]runtime.Object, error) {
	vsphereCluster := newVMWareCluster()
	cpMachineTemplate := newVMWareMachineTemplate(env.ClusterNameVar)
	workerMachineTemplate := newVMWareMachineTemplate(fmt.Sprintf("%s-worker", env.ClusterNameVar))
	controlPlane := newKubeadmControlplane(&cpMachineTemplate, nil)
	controlPlane.Spec.KubeadmConfigSpec.PreKubeadmCommands = append([]string{"dhclient eth0"}, controlPlane.Spec.KubeadmConfigSpec.PreKubeadmCommands...)
	kubevip.PatchControlPlane(&controlPlane)

	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s%s", env.ClusterNameVar, env.MachineDeploymentNameSuffix), true)
	kubeadmJoinTemplate.Spec.Template.Spec.PreKubeadmCommands = append([]string{"dhclient eth0"}, kubeadmJoinTemplate.Spec.Template.Spec.PreKubeadmCommands...)
	cluster := newCluster(&vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, &workerMachineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI, err := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	if err != nil {
		return nil, err
	}
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	identitySecret := newIdentitySecret()

	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&vsphereCluster,
		&cpMachineTemplate,
		&workerMachineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
		&clusterResourceSet,
		&identitySecret,
	}

	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)

	return MultiNodeTemplate, nil
}

func MultiNodeTemplateWithExternalLoadBalancer() ([]runtime.Object, error) {
	vsphereCluster := newVSphereCluster()
	cpMachineTemplate := newVSphereMachineTemplate(env.ClusterNameVar)
	workerMachineTemplate := newVSphereMachineTemplate(fmt.Sprintf("%s-worker", env.ClusterNameVar))
	controlPlane := newKubeadmControlplane(&cpMachineTemplate, nil)
	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s%s", env.ClusterNameVar, env.MachineDeploymentNameSuffix), true)
	cluster := newCluster(&vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, &workerMachineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI, err := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	if err != nil {
		return nil, err
	}
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	identitySecret := newIdentitySecret()

	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&vsphereCluster,
		&cpMachineTemplate,
		&workerMachineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
		&clusterResourceSet,
		&identitySecret,
	}
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)

	return MultiNodeTemplate, nil
}

func MultiNodeTemplateWithKubeVIPIgnition() ([]runtime.Object, error) {
	vsphereCluster := newVSphereCluster()
	machineTemplate := newVSphereMachineTemplate(env.ClusterNameVar)

	controlPlane := newIgnitionKubeadmControlplane(machineTemplate, nil)
	kubevip.PatchControlPlane(&controlPlane)

	// CABPK requires specifying file permissions in Ignition mode. Set a default value if not set.
	for i := range controlPlane.Spec.KubeadmConfigSpec.Files {
		if controlPlane.Spec.KubeadmConfigSpec.Files[i].Permissions == "" {
			controlPlane.Spec.KubeadmConfigSpec.Files[i].Permissions = "0400"
		}
	}

	kubeadmJoinTemplate := newIgnitionKubeadmConfigTemplate()
	cluster := newCluster(&vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, &machineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI, err := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	if err != nil {
		return nil, err
	}
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	identitySecret := newIdentitySecret()

	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&vsphereCluster,
		&machineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
		&clusterResourceSet,
		&identitySecret,
	}

	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)

	return MultiNodeTemplate, nil
}

func MultiNodeTemplateWithKubeVIPNodeIPAM() ([]runtime.Object, error) {
	vsphereCluster := newVSphereCluster()
	cpMachineTemplate := newNodeIPAMVSphereMachineTemplate(env.ClusterNameVar)
	workerMachineTemplate := newNodeIPAMVSphereMachineTemplate(fmt.Sprintf("%s-worker", env.ClusterNameVar))
	controlPlane := newKubeadmControlplane(&cpMachineTemplate, nil)
	kubevip.PatchControlPlane(&controlPlane)

	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s%s", env.ClusterNameVar, env.MachineDeploymentNameSuffix), true)
	cluster := newCluster(&vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, &workerMachineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI, err := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	if err != nil {
		return nil, err
	}
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	identitySecret := newIdentitySecret()

	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&vsphereCluster,
		&cpMachineTemplate,
		&workerMachineTemplate,
		&controlPlane,
		&kubeadmJoinTemplate,
		&machineDeployment,
		&clusterResourceSet,
		&identitySecret,
	}

	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)

	return MultiNodeTemplate, nil
}
