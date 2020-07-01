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

package kubeadmconfig

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"

	viputil "sigs.k8s.io/cluster-api-provider-vsphere/kube-vip/kubeadmconfig/util"
)

const (
	AdmissionPath = "/mutate-bootstrap-cluster-x-k8s-io-v1alpha3-kubeadmconfig"
)

type Webhook struct {
	client  ctrlclient.Client
	decoder *admission.Decoder
	Logger  logr.Logger
}

// +kubebuilder:webhook:verbs=create,path=/mutate-bootstrap-cluster-x-k8s-io-v1alpha3-kubeadmconfig,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs,versions=v1alpha3,name=mutation.kubeadmconfig.bootstrap.cluster.x-k8s.io,sideEffects=None
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vspheremachinetemplates,verbs=list;get;watch
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list

var _ admission.Handler = &Webhook{}

// Handle implements how to answer an admission request
func (r *Webhook) Handle(ctx context.Context, req admission.Request) admission.Response {

	kubeadmConfig := &bootstrapv1.KubeadmConfig{}

	if err := r.decoder.Decode(req, kubeadmConfig); err != nil {
		wrap := errors.Wrapf(err, "failed kubeadmconfig decode")
		return admission.Errored(http.StatusInternalServerError, wrap)
	}
	mutatedKubeadmConfig := kubeadmConfig.DeepCopy()

	// retrieve the KCP resource to be able to retrieve
	// the vsphereMachineTemplate. we can't only rely on machines
	// as kubeadmConfigs are generated before machines and infra machines
	kcp, err := viputil.GetKubeadmControlPlane(ctx, r.client, kubeadmConfig)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	// if it's not found, this means that the kubeadmConfig isn't for a
	// control plane machine
	if kcp == nil {
		return admission.Patched("kubeadmConfig not owned by a KubeadmControlPlane")
	}

	// get the kube-vip Pod from the kubeadmConfig file list
	vipPod, index, err := viputil.GetKubeVIPPod(kubeadmConfig)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	// if it's not found it means that this is for a cluster not using
	// kube-vip
	if vipPod == nil {
		return admission.Patched("kubeadmConfig not using kube-vip")
	}

	networkInterface, err := viputil.NetworkInterface(ctx, r.client, kcp)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	vipEnvs, containerIndex := viputil.KubeVIPEnvs(vipPod.Spec.Containers, networkInterface)
	if containerIndex < 0 {
		return admission.Patched("kube-vip pod doesn't have a kube-vip container")
	}

	vipPod.Spec.Containers[containerIndex].Env = vipEnvs
	kubeadmconfigBytes, err := viputil.MutateKubeadmConfig(mutatedKubeadmConfig, index, vipPod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	r.Logger.Info("kubeadmConfig marshalled", "kubeadmConfig", string(kubeadmconfigBytes))
	return admission.PatchResponseFromRaw(req.Object.Raw, kubeadmconfigBytes)
}

var _ inject.Client = &Webhook{}

func (r *Webhook) InjectClient(c ctrlclient.Client) error {
	r.client = c
	return nil
}

func (r *Webhook) InjectDecoder(d *admission.Decoder) error {
	r.decoder = d
	return nil
}
