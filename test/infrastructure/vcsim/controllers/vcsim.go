/*
Copyright 2024 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/govc/cli"
	"github.com/vmware/govmomi/simulator"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

// vcsim objects.

const (
	vcsimMinVersionForCAPV        = "7.0.0"
	vcsimDefaultNetworkName       = "VM Network"
	vcsimDefaultStoragePolicyName = "vSAN Default Storage Policy"

	// Note: for the sake of testing with vcsim the template doesn't really matter (nor the version of K8s hosted on it)
	// so we create only a VM template with a well-known name.
	vcsimDefaultVMTemplateName = "ubuntu-2204-kube-vX"
)

func vcsimDatacenterName(datacenter int) string {
	return fmt.Sprintf("DC%d", datacenter)
}

func vcsimClusterName(datacenter, cluster int) string {
	return fmt.Sprintf("%s_C%d", vcsimDatacenterName(datacenter), cluster)
}

func vcsimClusterPath(datacenter, cluster int) string {
	return fmt.Sprintf("/%s/host/%s", vcsimDatacenterName(datacenter), vcsimClusterName(datacenter, cluster))
}

func vcsimDatastoreName(datastore int) string {
	return fmt.Sprintf("LocalDS_%d", datastore)
}

func vcsimDatastorePath(datacenter, datastore int) string {
	return fmt.Sprintf("/%s/datastore/%s", vcsimDatacenterName(datacenter), vcsimDatastoreName(datastore))
}

func vcsimResourcePoolPath(datacenter, cluster int) string {
	return fmt.Sprintf("/%s/host/%s/Resources", vcsimDatacenterName(datacenter), vcsimClusterName(datacenter, cluster))
}

func vcsimVMFolderName(datacenter int) string {
	return fmt.Sprintf("%s/vm", vcsimDatacenterName(datacenter))
}

func vcsimVMPath(datacenter int, vm string) string {
	return fmt.Sprintf("/%s/%s", vcsimVMFolderName(datacenter), vm)
}

func createVMTemplate(ctx context.Context, vCenterSimulator *vcsimv1.VCenterSimulator) error {
	log := ctrl.LoggerFrom(ctx)
	govcURL := fmt.Sprintf("https://%s:%s@%s/sdk", vCenterSimulator.Status.Username, vCenterSimulator.Status.Password, vCenterSimulator.Status.Host)

	// TODO: Investigate how template are supposed to work
	//  we create a template in a datastore, what if many?
	//  we create a template in a cluster, but the generated vm doesn't have the cluster in the path. What if I have many clusters?
	cluster := 0
	datastore := 0
	datacenters := 1
	if vCenterSimulator.Spec.Model != nil {
		datacenters = int(ptr.Deref(vCenterSimulator.Spec.Model.Datacenter, int32(simulator.VPX().Datacenter))) // VPX is the same base model used when creating vcsim
	}
	for dc := 0; dc < datacenters; dc++ {
		exit := cli.Run([]string{"vm.create", fmt.Sprintf("-ds=%s", vcsimDatastoreName(datastore)), fmt.Sprintf("-cluster=%s", vcsimClusterName(dc, cluster)), fmt.Sprintf("-net=%s", vcsimDefaultNetworkName), "-disk=20G", "-on=false", "-k=true", fmt.Sprintf("-u=%s", govcURL), vcsimDefaultVMTemplateName})
		if exit != 0 {
			return errors.New("failed to create vm template")
		}

		exit = cli.Run([]string{"vm.markastemplate", "-k=true", fmt.Sprintf("-u=%s", govcURL), vcsimVMPath(dc, vcsimDefaultVMTemplateName)})
		if exit != 0 {
			return errors.New("failed to mark vm template")
		}
		log.Info("Created VM template", "name", vcsimDefaultVMTemplateName)
	}
	return nil
}
