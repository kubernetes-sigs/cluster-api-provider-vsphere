/*
Copyright 2021 The Kubernetes Authors.

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
	goctx "context"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/cluster"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/metadata"
)

func (r vsphereDeploymentZoneReconciler) reconcileFailureDomain(ctx *context.VSphereDeploymentZoneContext) error {
	logger := ctrl.LoggerFrom(ctx).WithValues("failure domain", ctx.VSphereFailureDomain.Name)

	// verify the failure domain for the region
	if err := r.reconcileInfraFailureDomain(ctx, ctx.VSphereFailureDomain.Spec.Region); err != nil {
		conditions.MarkFalse(ctx.VSphereDeploymentZone, infrav1.VSphereFailureDomainConfigurationCondition, infrav1.RegionMisconfiguredReason, clusterv1.ConditionSeverityError, err.Error())
		logger.Error(err, "region is not configured correctly")
		return errors.Wrapf(err, "region is not configured correctly")
	}

	// verify the failure domain for the zone
	if err := r.reconcileInfraFailureDomain(ctx, ctx.VSphereFailureDomain.Spec.Zone); err != nil {
		conditions.MarkFalse(ctx.VSphereDeploymentZone, infrav1.VSphereFailureDomainConfigurationCondition, infrav1.ZoneMisconfiguredReason, clusterv1.ConditionSeverityError, err.Error())
		logger.Error(err, "zone is not configured correctly")
		return errors.Wrapf(err, "zone is not configured correctly")
	}

	if computeCluster := ctx.VSphereFailureDomain.Spec.Topology.ComputeCluster; computeCluster != nil {
		if err := r.reconcileComputeCluster(ctx); err != nil {
			conditions.MarkFalse(ctx.VSphereDeploymentZone, infrav1.VSphereFailureDomainConfigurationCondition, infrav1.ComputeClusterMisconfiguredReason, clusterv1.ConditionSeverityError, "compute cluster %s is misconfigured", *computeCluster)
			logger.Error(err, "compute cluster is not configured correctly")
			return errors.Wrap(err, "compute cluster is not configured correctly")
		}
	}

	if err := r.reconcileTopology(ctx); err != nil {
		logger.Error(err, "topology is not configured correctly")
		return errors.Wrap(err, "topology is not configured correctly")
	}
	return nil
}

func (r vsphereDeploymentZoneReconciler) reconcileInfraFailureDomain(ctx *context.VSphereDeploymentZoneContext, failureDomain infrav1.FailureDomain) error {
	if autoConfigure := pointer.BoolDeref(failureDomain.AutoConfigure, false); autoConfigure {
		return r.createAndAttachMetadata(ctx, failureDomain)
	}
	return r.verifyFailureDomain(ctx, failureDomain)
}

func (r vsphereDeploymentZoneReconciler) reconcileTopology(ctx *context.VSphereDeploymentZoneContext) error {
	topology := ctx.VSphereFailureDomain.Spec.Topology
	if datastore := topology.Datastore; datastore != "" {
		if _, err := ctx.AuthSession.Finder.Datastore(ctx, datastore); err != nil {
			ctx.Logger.V(4).Error(err, "unable to find datastore", "name", datastore)
			conditions.MarkFalse(ctx.VSphereDeploymentZone, infrav1.PlacementConstraintConfigurationCondition, infrav1.DatastoreMisconfiguredReason, clusterv1.ConditionSeverityError, "datastore %s is misconfigured", datastore)
			return errors.Wrapf(err, "unable to find datastore %s", datastore)
		}
	}

	for _, network := range topology.Networks {
		if _, err := ctx.AuthSession.Finder.Network(ctx, network); err != nil {
			ctx.Logger.V(4).Error(err, "unable to find network", "name", network)
			conditions.MarkFalse(ctx.VSphereDeploymentZone, infrav1.PlacementConstraintConfigurationCondition, infrav1.NetworkMisconfiguredReason, clusterv1.ConditionSeverityError, "network %s is misconfigured", network)
			return errors.Wrapf(err, "unable to find network %s", network)
		}
	}

	if hostPlacementInfo := topology.Hosts; hostPlacementInfo != nil {
		rule, err := cluster.VerifyAffinityRule(ctx, *topology.ComputeCluster, hostPlacementInfo.HostGroupName, hostPlacementInfo.VMGroupName)
		switch {
		case err != nil:
			ctx.Logger.V(4).Error(err, "unable to find affinity rule")
			conditions.MarkFalse(ctx.VSphereDeploymentZone, infrav1.VSphereFailureDomainConfigurationCondition, infrav1.HostsMisconfiguredReason, clusterv1.ConditionSeverityError, "vm host affinity does not exist")
			return err
		case rule.Disabled():
			ctx.Logger.V(4).Info("warning: vm-host rule for the failure domain is disabled")
			conditions.MarkFalse(ctx.VSphereDeploymentZone, infrav1.VSphereFailureDomainConfigurationCondition, infrav1.HostsAffinityMisconfiguredReason, clusterv1.ConditionSeverityWarning, "vm host affinity is disabled")
		default:
			conditions.MarkTrue(ctx.VSphereDeploymentZone, infrav1.VSphereFailureDomainConfigurationCondition)
		}
	}
	return nil
}

func (r vsphereDeploymentZoneReconciler) reconcileComputeCluster(ctx *context.VSphereDeploymentZoneContext) error {
	if computeCluster := ctx.VSphereFailureDomain.Spec.Topology.ComputeCluster; computeCluster != nil {
		ccr, err := ctx.AuthSession.Finder.ClusterComputeResource(ctx, *computeCluster)
		if err != nil {
			ctx.Logger.V(4).Error(err, "unable to find resource pool", "name", ctx.VSphereDeploymentZone.Spec.PlacementConstraint.ResourcePool)
			return errors.Wrap(err, "unable to find resource pool")
		}

		if resourcePool := ctx.VSphereDeploymentZone.Spec.PlacementConstraint.ResourcePool; resourcePool != "" {
			rp, err := ctx.AuthSession.Finder.ResourcePool(ctx, ctx.VSphereDeploymentZone.Spec.PlacementConstraint.ResourcePool)
			if err != nil {
				ctx.Logger.V(4).Error(err, "unable to find resource pool", "name", ctx.VSphereDeploymentZone.Spec.PlacementConstraint.ResourcePool)
				return errors.Wrapf(err, "unable to find resource pool")
			}

			ref, err := rp.Owner(ctx)
			if err != nil {
				ctx.Logger.V(4).Error(err, "unable to find owner compute resource", "name", ctx.VSphereDeploymentZone.Spec.PlacementConstraint.ResourcePool)
				return errors.Wrap(err, "unable to find resource pool")
			}

			if ref.Reference() != ccr.Reference() {
				ctx.Logger.V(4).Error(nil, "compute cluster does not own resource pool", "computeCluster", computeCluster, "resourcePool", ctx.VSphereDeploymentZone.Spec.PlacementConstraint.ResourcePool)
				return errors.Errorf("compute cluster %s does not own resource pool %s", *computeCluster, resourcePool)
			}
		}
	}
	return nil
}

// verifyFailureDomain verifies the Failure Domain. It verifies the existence of tag and category specified and
// checks whether the specified tags exist on the DataCenter or Compute Cluster or Hosts (in a HostGroup).
func (r vsphereDeploymentZoneReconciler) verifyFailureDomain(ctx *context.VSphereDeploymentZoneContext, failureDomain infrav1.FailureDomain) error {
	if _, err := ctx.AuthSession.TagManager.GetTagForCategory(ctx, failureDomain.Name, failureDomain.TagCategory); err != nil {
		return errors.Wrapf(err, "failed to verify tag %s and category %s", failureDomain.Name, failureDomain.TagCategory)
	}

	managedRefFunc := getManagedRefFinder(failureDomain, ctx.VSphereFailureDomain.Spec.Topology, ctx.AuthSession.Finder)
	managedRefs, err := managedRefFunc(ctx)
	if err != nil {
		return err
	}

TagVerifier:
	for _, ref := range managedRefs {
		attachedTags, err := ctx.AuthSession.TagManager.GetAttachedTags(ctx, ref)
		if err != nil {
			return err
		}
		for _, tag := range attachedTags {
			if tag.Name == failureDomain.Name {
				continue TagVerifier
			}
		}
		return errors.Errorf("tag %s is not associated to %s", failureDomain.Name, ref.Reference().Type)
	}
	return nil
}

func (r vsphereDeploymentZoneReconciler) createAndAttachMetadata(ctx *context.VSphereDeploymentZoneContext, failureDomain infrav1.FailureDomain) error {
	categoryID, err := metadata.CreateCategory(ctx, failureDomain.TagCategory, failureDomain.Type)
	if err != nil {
		return errors.Wrapf(err, "failed to create category %s", failureDomain.TagCategory)
	}
	err = metadata.CreateTag(ctx, failureDomain.Name, categoryID)
	if err != nil {
		return errors.Wrapf(err, "failed to create tag %s", failureDomain.Name)
	}

	managedRefFunc := getManagedRefFinder(failureDomain, ctx.VSphereFailureDomain.Spec.Topology, ctx.AuthSession.Finder)
	managedRefs, err := managedRefFunc(ctx)
	if err != nil || len(managedRefs) == 0 {
		return err
	}

	managedRef, err := managedRefFunc(ctx)
	if err != nil {
		return err
	}

	err = ctx.AuthSession.TagManager.AttachTag(ctx, failureDomain.Name, managedRef[0])
	if err != nil {
		err = errors.Wrapf(err, "failed to attach tag %s to region", failureDomain.Name)
	}
	return err
}

// managedRefFinder is the method to find the reference of the type specified in the Failure Domain
type managedRefFinder func(goctx.Context) ([]object.Reference, error)

// getManagedRefFinder returns the method to find the reference specified in the Failure Domain
func getManagedRefFinder(failureDomain infrav1.FailureDomain, topology infrav1.Topology, finder *find.Finder) managedRefFinder {
	var managedRefFunc managedRefFinder

	switch failureDomain.Type {
	case infrav1.ComputeClusterFailureDomain:
		managedRefFunc = func(ctx goctx.Context) ([]object.Reference, error) {
			computeResource, err := finder.ClusterComputeResource(ctx, *topology.ComputeCluster)
			if err != nil {
				return nil, err
			}
			return []object.Reference{computeResource.Reference()}, nil
		}
	case infrav1.DatacenterFailureDomain:
		managedRefFunc = func(ctx goctx.Context) ([]object.Reference, error) {
			dataCenter, err := finder.Datacenter(ctx, topology.Datacenter)
			if err != nil {
				return nil, err
			}
			return []object.Reference{dataCenter.Reference()}, nil
		}
	case infrav1.HostGroupFailureDomain:
		managedRefFunc = func(ctx goctx.Context) ([]object.Reference, error) {
			ccr, err := finder.ClusterComputeResource(ctx, *topology.ComputeCluster)
			if err != nil {
				return nil, err
			}
			return cluster.ListHostsFromGroup(ctx, ccr, topology.Hosts.HostGroupName)
		}
	}
	return managedRefFunc
}
