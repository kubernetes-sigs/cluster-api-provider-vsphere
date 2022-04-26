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

package flavors

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/crs"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
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

func ClusterTopologyTemplateKubeVIP() []runtime.Object {
	cluster := newClusterClassCluster()
	identitySecret := newIdentitySecret()
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
	crsResourcesCPI := crs.CreateCrsResourceObjectsCPI(&clusterResourceSet)
	MultiNodeTemplate := []runtime.Object{
		&cluster,
		&identitySecret,
		&clusterResourceSet,
	}
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCSI...)
	MultiNodeTemplate = append(MultiNodeTemplate, crsResourcesCPI...)
	return MultiNodeTemplate
}

func MultiNodeTemplateWithKubeVIP() []runtime.Object {
	vsphereCluster := newVSphereCluster()
	machineTemplate := newVSphereMachineTemplate(env.ClusterNameVar)
	controlPlane := newKubeadmControlplane(444, machineTemplate, newKubeVIPFiles())
	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s%s", env.ClusterNameVar, env.MachineDeploymentNameSuffix), true)
	cluster := newCluster(vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, machineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
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

	return MultiNodeTemplate
}

func MultiNodeTemplateWithExternalLoadBalancer() []runtime.Object {
	vsphereCluster := newVSphereCluster()
	machineTemplate := newVSphereMachineTemplate(env.ClusterNameVar)
	controlPlane := newKubeadmControlplane(444, machineTemplate, nil)
	kubeadmJoinTemplate := newKubeadmConfigTemplate(fmt.Sprintf("%s%s", env.ClusterNameVar, env.MachineDeploymentNameSuffix), true)
	cluster := newCluster(vsphereCluster, &controlPlane)
	machineDeployment := newMachineDeployment(cluster, machineTemplate, kubeadmJoinTemplate)
	clusterResourceSet := newClusterResourceSet(cluster)
	crsResourcesCSI := crs.CreateCrsResourceObjectsCSI(&clusterResourceSet)
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

	return MultiNodeTemplate
}
