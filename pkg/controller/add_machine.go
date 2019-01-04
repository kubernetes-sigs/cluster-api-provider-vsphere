/*
Copyright 2018 The Kubernetes Authors.

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

package controller

import (
	"github.com/golang/glog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/cluster-api/pkg/controller/machine"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, func(m manager.Manager) error {
		factory := getSharedInformerFactory(m)
		informer := factory.Cluster().V1alpha1()

		client, err := clientset.NewForConfig(m.GetConfig())
		if err != nil {
			glog.Fatalf("Failed to create clientset: %v", err)
		}

		machineClientSet, err := kubernetes.NewForConfig(
			rest.AddUserAgent(m.GetConfig(), "machine-controller-manager"),
		)
		if err != nil {
			glog.Fatalf("Failed to create client: %v", err)
		}

		machineEventRecorder, err := createRecorder(machineClientSet, "machine-controller-manager")
		if err != nil {
			glog.Fatalf("Could not create vSphere event recorder: %v", err)
		}

		//TODO: remove need for client
		actuator, err := vsphere.NewGovmomiMachineActuator(m, client.ClusterV1alpha1(), machineClientSet, informer, machineEventRecorder)
		if err != nil {
			glog.Fatalf("Could not create vSphere machine actuator: %v", err)
		}

		return machine.AddWithActuator(m, actuator)
	})
}
