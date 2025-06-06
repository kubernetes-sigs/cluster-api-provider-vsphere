/*
Copyright 2019 The Kubernetes Authors.

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

package fake

import (
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1beta1 "sigs.k8s.io/cluster-api/api/ipam/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

// NewControllerManagerContext returns a fake ControllerManagerContext for unit
// testing reconcilers and webhooks with a fake client. You can choose to
// initialize it with a slice of runtime.Object.

func NewControllerManagerContext(initObjects ...client.Object) *capvcontext.ControllerManagerContext {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = vmwarev1.AddToScheme(scheme)
	_ = vmoprv1.AddToScheme(scheme)
	_ = ipamv1beta1.AddToScheme(scheme)

	clientWithObjects := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(
		&infrav1.VSphereVM{},
		&vmwarev1.VSphereCluster{},
		&clusterv1.Cluster{},
	).WithObjects(initObjects...).Build()

	return &capvcontext.ControllerManagerContext{
		Client:                  clientWithObjects,
		Logger:                  ctrllog.Log.WithName(ControllerManagerName),
		Scheme:                  scheme,
		Namespace:               ControllerManagerNamespace,
		Name:                    ControllerManagerName,
		LeaderElectionNamespace: LeaderElectionNamespace,
		LeaderElectionID:        LeaderElectionID,
	}
}
