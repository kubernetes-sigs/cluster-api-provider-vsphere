package tags

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/types"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

// NetApp
const (
	categoryName               = "NKS"
	clusterInfoTagNameTemplate = "nks.workspaceid.%s.clusterid.%s.clustername.%s"
	serviceClusterTagName      = "nks.service.cluster"
)

// NetApp
// TagNKSMachine tags the machine with NKS vSphere tags. If the tags do not exist, they are created.
func TagNKSMachine(ctx *context.MachineContext, vm *object.VirtualMachine) error {

	tagManager := tags.NewManager(ctx.RestSession.Client)
	clusterID, workspaceID, isServiceCluster := ctx.GetNKSClusterInfo()

	ctx.Logger.V(4).Info("tagging VM with cluster information")
	if err := tagWithClusterInfo(ctx, tagManager, vm.Reference(), workspaceID, clusterID, ctx.Cluster.Name); err != nil {
		return errors.Wrapf(err, "could not tag VM with cluster information for machine %q", ctx.Machine.Name)
	}

	if isServiceCluster {
		ctx.Logger.V(4).Info("tagging VM as service cluster machine")
		if err := tagAsServiceCluster(ctx, tagManager, vm.Reference()); err != nil {
			return errors.Wrapf(err, "could not tag VM as service cluster machine for machine %q", ctx.Machine.Name)
		}
	}

	return nil
}

// NetApp
// CleanupNKSTags deletes vSphere tags and tag categories that may be associated with the machine - if they are not attached to any objects anymore
func CleanupNKSTags(ctx *context.MachineContext) error {

	tagManager := tags.NewManager(ctx.RestSession.Client)
	clusterID, workspaceID, isServiceCluster := ctx.GetNKSClusterInfo()

	ctx.Logger.V(4).Info("cleaning up cluster information tag and category", "cluster", ctx.Cluster.Name)
	if err := deleteClusterInfoTagAndCategory(ctx, tagManager, workspaceID, clusterID, ctx.Cluster.Name); err != nil {
		return errors.Wrapf(err, "could not clean up cluster information tag and category for cluster %q", ctx.Cluster.Name)
	}

	if isServiceCluster {
		ctx.Logger.V(4).Info("cleaning up service cluster tag and category", "cluster", ctx.Cluster.Name)
		if err := deleteServiceClusterTagAndCategory(ctx, tagManager); err != nil {
			return errors.Wrapf(err, "could not clean up service cluster tag and category for cluster %q", ctx.Cluster.Name)
		}
	}

	return nil
}

// NetApp
func tagWithClusterInfo(ctx *context.MachineContext, tm *tags.Manager, moref types.ManagedObjectReference, workspaceID string, clusterID string, clusterName string) error {

	tagName := fmt.Sprintf(clusterInfoTagNameTemplate, workspaceID, clusterID, clusterName)

	tag, err := getOrCreateNKSTag(ctx, tm, tagName)
	if err != nil {
		return err
	}

	err = tm.AttachTag(ctx, tag.ID, moref)
	if err != nil {
		return errors.Wrapf(err, "could not attach tag %s to object", tag.Name)
	}

	return nil
}

// NetApp
// deleteClusterInfoTagAndCategory deletes the cluster info tag if there are no subjects tied to the tag, i.e. no objects are tagged with that tag
// It also deletes the tag category if there are no tags left in the category
func deleteClusterInfoTagAndCategory(ctx *context.MachineContext, tm *tags.Manager, workspaceID string, clusterID string, clusterName string) error {

	tagName := fmt.Sprintf(clusterInfoTagNameTemplate, workspaceID, clusterID, clusterName)

	tag, err := tm.GetTag(ctx, tagName)
	if err != nil {
		return errors.Wrapf(err, "could not get tag with name %s", tagName)
	}

	return deleteNKSTag(ctx, tm, tag)
}

// NetApp
func tagAsServiceCluster(ctx *context.MachineContext, tm *tags.Manager, moref types.ManagedObjectReference) error {

	tag, err := getOrCreateNKSTag(ctx, tm, serviceClusterTagName)
	if err != nil {
		return err
	}

	err = tm.AttachTag(ctx, tag.ID, moref)
	if err != nil {
		return errors.Wrapf(err, "could not attach tag %s to object", tag.Name)
	}

	return nil
}

// NetApp
// deleteServiceClusterTagAndCategory deletes the service cluster tag if there are no subjects tied to the tag, i.e. no objects are tagged with that tag
// It also deletes the tag category if there are no tags left in the category
func deleteServiceClusterTagAndCategory(ctx *context.MachineContext, tm *tags.Manager) error {

	tag, err := tm.GetTag(ctx, serviceClusterTagName)
	if err != nil {
		return errors.Wrapf(err, "could not get tag with name %s", serviceClusterTagName)
	}

	return deleteNKSTag(ctx, tm, tag)
}

// NetApp
func getOrCreateNKSTag(ctx *context.MachineContext, tm *tags.Manager, tagName string) (*tags.Tag, error) {

	tag, err := tm.GetTag(ctx, tagName)
	if err == nil && tag != nil {
		return tag, nil
	}

	nksCategory, err := getOrCreateNKSTagCategory(ctx, tm)
	if err != nil {
		return nil, errors.Wrap(err, "could not get NKS tag category")
	}

	newTag := &tags.Tag{
		Name:        tagName,
		Description: "NKS tag",
		CategoryID:  nksCategory.ID,
	}

	ctx.Logger.V(4).Info("creating vSphere tag", "tag", newTag.Name)
	_, err = tm.CreateTag(ctx, newTag)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create tag %s", newTag)
	}

	tag, err = tm.GetTag(ctx, tagName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get tag with name %s", tagName)
	}

	return tag, nil
}

// NetApp
func getOrCreateNKSTagCategory(ctx *context.MachineContext, tm *tags.Manager) (*tags.Category, error) {

	category, err := tm.GetCategory(ctx, categoryName)
	if err == nil && category != nil {
		return category, nil
	}

	newCategory := &tags.Category{
		Name:        categoryName,
		Description: "NKS tag category",
		Cardinality: "MULTIPLE",
		AssociableTypes: []string{
			"Folder",
			"VirtualMachine",
		},
	}

	ctx.Logger.V(4).Info("creating vSphere tag category", "category", newCategory.Name)
	_, err = tm.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create tag category %s", newCategory)
	}

	category, err = tm.GetCategory(ctx, categoryName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get tag category with name %s", categoryName)
	}

	return category, nil
}

// NetApp
func deleteNKSTag(ctx *context.MachineContext, tm *tags.Manager, tag *tags.Tag) error {

	attachedObjects, err := tm.ListAttachedObjects(ctx, tag.ID)
	if err != nil {
		return errors.Wrapf(err, "could not list attached objects for tag with name %s", tag.Name)
	}

	if len(attachedObjects) != 0 {
		ctx.Logger.V(4).Info("will not delete vSphere tag, still in use", "tag", tag.Name)
		return nil
	}

	ctx.Logger.V(4).Info("deleting vSphere tag", "tag", tag.Name)
	err = tm.DeleteTag(ctx, tag)
	if err != nil {
		return errors.Wrapf(err, "could not delete tag with name %s")
	}

	category, err := tm.GetCategory(ctx, tag.CategoryID)
	if err != nil {
		return errors.Wrapf(err, "could not get category with ID %s", tag.CategoryID)
	}

	err = deleteNKSTagCategory(ctx, tm, category)
	if err != nil {
		return errors.Wrapf(err, "could not delete category for tag %s with name %s", tag.Name, category.Name)
	}

	return nil

}

// NetApp
func deleteNKSTagCategory(ctx *context.MachineContext, tm *tags.Manager, category *tags.Category) error {

	tagsInCategory, err := tm.GetTagsForCategory(ctx, category.Name)
	if err != nil {
		return errors.Wrapf(err, "could not get tags for category with name %s", category.Name)
	}

	if len(tagsInCategory) != 0 {
		ctx.Logger.V(4).Info("will not delete vSphere tag category, still in use", "category", category.Name)
		return nil
	}

	ctx.Logger.V(4).Info("deleting vSphere tag category", "category", category.Name)
	err = tm.DeleteCategory(ctx, category)
	if err != nil {
		return errors.Wrapf(err, "could not delete category with name %s", category.Name)
	}
	return nil

}