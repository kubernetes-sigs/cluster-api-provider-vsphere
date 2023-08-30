/*
Copyright 2022 The Kubernetes Authors.

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

package clustermodule

import (
	"context"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/types"

	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/clustermodules"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

const validMachineTemplate = "VSphereMachineTemplate"

type service struct{}

func NewService() Service {
	return service{}
}

func (s service) Create(clusterCtx *capvcontext.ClusterContext, wrapper Wrapper) (string, error) {
	logger := clusterCtx.Logger.WithValues("object", wrapper.GetName(), "namespace", wrapper.GetNamespace())

	templateRef, err := fetchTemplateRef(clusterCtx, clusterCtx.Client, wrapper)
	if err != nil {
		logger.V(4).Error(err, "error fetching template for object")
		return "", errors.Wrapf(err, "error fetching machine template for object %s/%s", wrapper.GetNamespace(), wrapper.GetName())
	}
	if templateRef.Kind != validMachineTemplate {
		// since this is a heterogeneous cluster, we should skip cluster module creation for non VSphereMachine objects
		logger.V(4).Info("skipping module creation for object")
		return "", nil
	}

	template, err := fetchMachineTemplate(clusterCtx, wrapper, templateRef.Name)
	if err != nil {
		logger.V(4).Error(err, "error fetching template")
		return "", err
	}
	if server := template.Spec.Template.Spec.Server; server != clusterCtx.VSphereCluster.Spec.Server {
		logger.V(4).Info("skipping module creation for object since template uses a different server", "server", server)
		return "", nil
	}

	vCenterSession, err := fetchSessionForObject(clusterCtx, template)
	if err != nil {
		logger.V(4).Error(err, "error fetching session")
		return "", err
	}

	// Fetch the compute cluster resource by tracing the owner of the resource pool in use.
	// TODO (srm09): How do we support Multi AZ scenarios here
	computeClusterRef, err := getComputeClusterResource(clusterCtx, vCenterSession, template.Spec.Template.Spec.ResourcePool)
	if err != nil {
		logger.V(4).Error(err, "error fetching compute cluster resource")
		return "", err
	}

	provider := clustermodules.NewProvider(vCenterSession.TagManager.Client)
	moduleUUID, err := provider.CreateModule(clusterCtx, computeClusterRef)
	if err != nil {
		logger.V(4).Error(err, "error creating cluster module")
		return "", err
	}
	logger.V(4).Info("created cluster module for object", "moduleUUID", moduleUUID)
	return moduleUUID, nil
}

func (s service) DoesExist(clusterCtx *capvcontext.ClusterContext, wrapper Wrapper, moduleUUID string) (bool, error) {
	logger := clusterCtx.Logger.WithValues("object", wrapper.GetName())

	templateRef, err := fetchTemplateRef(clusterCtx, clusterCtx.Client, wrapper)
	if err != nil {
		logger.V(4).Error(err, "error fetching template for object")
		return false, errors.Wrapf(err, "error fetching infrastructure machine template for object %s/%s", wrapper.GetNamespace(), wrapper.GetName())
	}

	template, err := fetchMachineTemplate(clusterCtx, wrapper, templateRef.Name)
	if err != nil {
		logger.V(4).Error(err, "error fetching template")
		return false, err
	}

	vCenterSession, err := fetchSessionForObject(clusterCtx, template)
	if err != nil {
		logger.V(4).Error(err, "error fetching session")
		return false, err
	}

	// Fetch the compute cluster resource by tracing the owner of the resource pool in use.
	// TODO (srm09): How do we support Multi AZ scenarios here
	computeClusterRef, err := getComputeClusterResource(clusterCtx, vCenterSession, template.Spec.Template.Spec.ResourcePool)
	if err != nil {
		logger.V(4).Error(err, "error fetching compute cluster resource")
		return false, err
	}

	provider := clustermodules.NewProvider(vCenterSession.TagManager.Client)
	return provider.DoesModuleExist(clusterCtx, moduleUUID, computeClusterRef)
}

func (s service) Remove(clusterCtx *capvcontext.ClusterContext, moduleUUID string) error {
	params := newParams(*clusterCtx)
	vcenterSession, err := fetchSession(clusterCtx, params)
	if err != nil {
		return err
	}

	provider := clustermodules.NewProvider(vcenterSession.TagManager.Client)
	return provider.DeleteModule(clusterCtx, moduleUUID)
}

func getComputeClusterResource(ctx context.Context, s *session.Session, resourcePool string) (types.ManagedObjectReference, error) {
	rp, err := s.Finder.ResourcePoolOrDefault(ctx, resourcePool)
	if err != nil {
		return types.ManagedObjectReference{}, err
	}

	cc, err := rp.Owner(ctx)
	if err != nil {
		return types.ManagedObjectReference{}, err
	}

	ownerPath, err := find.InventoryPath(ctx, s.Client.Client, cc.Reference())
	if err != nil {
		return types.ManagedObjectReference{}, err
	}
	if _, err = s.Finder.ClusterComputeResource(ctx, ownerPath); err != nil {
		return types.ManagedObjectReference{}, IncompatibleOwnerError{cc.Reference().Value}
	}

	return cc.Reference(), nil
}
