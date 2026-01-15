/*
Copyright 2026 The Kubernetes Authors.

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

package client

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	vmoprv1alpha2 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion"
	vmoprvhub "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/hub"
	vmoprv1alpha2conversion "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/v1alpha2"
	vmoprv1alpha5conversion "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/vmoperator/v1alpha5"
)

// DefaultConverter is a converter aware of the API types and the conversions defined in sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api.
var DefaultConverter *conversion.Converter

func init() {
	DefaultConverter = conversion.NewConverter()

	utilruntime.Must(vmoprvhub.AddToConverter(DefaultConverter))
	utilruntime.Must(vmoprv1alpha2conversion.AddToConverter(DefaultConverter))
	utilruntime.Must(vmoprv1alpha5conversion.AddToConverter(DefaultConverter))

	// TODO: Add dynamic selection of target version.
	DefaultConverter.SetTargetVersion(vmoprv1alpha2.GroupVersion.Version)
}

// New return a client that can convert before write and after read using a DefaultConverter.
func New(c client.Client) (client.Client, error) {
	return NewWithConverter(c, DefaultConverter)
}

// NewWithConverter return a client that can convert objects before write and after read.
func NewWithConverter(c client.Client, converter *conversion.Converter) (client.Client, error) {
	if err := checkConverterAndSchemeAreConsistent(converter, c.Scheme()); err != nil {
		return nil, err
	}

	return &conversionClient{
		internalClient: c,
		converter:      converter,
	}, nil
}

func checkConverterAndSchemeAreConsistent(converter *conversion.Converter, scheme *runtime.Scheme) error {
	allErrs := []error{}
	for gvk := range converter.AllKnownTypes() {
		if strings.HasSuffix(gvk.Kind, "List") {
			continue
		}

		if !scheme.Recognizes(gvk) {
			allErrs = append(allErrs, errors.Errorf("converter is configured to handle %s but the client scheme is not aware of this type", gvk))
		}

		for _, targetGV := range scheme.PrioritizedVersionsForGroup(gvk.Group) {
			if targetGV == gvk.GroupVersion() {
				continue
			}
			targetGVK := targetGV.WithKind(gvk.Kind)

			if !scheme.Recognizes(targetGVK) {
				continue
			}
			if !converter.Recognizes(targetGVK) {
				allErrs = append(allErrs, errors.Errorf("converter is configured to handle %s but it is not configured for handling conversions from/to %s which is registered in the client scheme", gvk, targetGV.Version))
			}
		}
	}
	return kerrors.NewAggregate(allErrs)
}

type conversionClient struct {
	converter      *conversion.Converter
	internalClient client.Client
}

// conversionClient must implement client.Client.
var _ client.Client = &conversionClient{}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
func (c *conversionClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if !c.converter.IsConvertible(obj) {
		return c.internalClient.Get(ctx, key, obj, opts...)
	}

	targetVersionObj, err := c.newTargetVersionObjectFor(obj)
	if err != nil {
		return err
	}

	if err := c.internalClient.Get(ctx, key, targetVersionObj, opts...); err != nil {
		return err
	}
	if err := c.converter.Convert(targetVersionObj, obj); err != nil {
		return errors.Wrapf(err, "failed to convert %s from target version while getting it", klog.KObj(targetVersionObj))
	}
	return nil
}

// List retrieves list of objects for a given namespace and list options.
func (c *conversionClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if !c.converter.IsConvertible(list) {
		return c.internalClient.List(ctx, list, opts...)
	}

	targetList, err := c.newTargetVersionObjectListFor(list)
	if err != nil {
		return err
	}

	if err := c.internalClient.List(ctx, targetList, opts...); err != nil {
		return err
	}

	targetItems, err := meta.ExtractList(targetList)
	if err != nil {
		return err
	}

	listObjs := []runtime.Object{}
	for _, targetItemRaw := range targetItems {
		targetItem, ok := targetItemRaw.(client.Object)
		if !ok {
			return errors.Errorf("%T does not implement client.Object", targetItemRaw)
		}

		listItem, err := c.newObjectListItemFor(list)
		if err != nil {
			return err
		}

		if err := c.converter.Convert(targetItem, listItem); err != nil {
			return errors.Wrapf(err, "failed to convert %s from target version while listing it", klog.KObj(listItem))
		}
		listObjs = append(listObjs, listItem)
	}

	return meta.SetList(list, listObjs)
}

// Apply applies the given apply configuration to the Kubernetes cluster.
func (c *conversionClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	cObj, ok := obj.(client.Object)
	if !ok {
		return errors.Errorf("%T does not implement client.Object", obj)
	}

	if !c.converter.IsConvertible(cObj) {
		return c.internalClient.Apply(ctx, obj, opts...)
	}

	// conversionClient only implements methods used in CAPV.
	panic("implement me")
}

// Create saves the object obj in the Kubernetes cluster.
func (c *conversionClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if !c.converter.IsConvertible(obj) {
		return c.internalClient.Create(ctx, obj, opts...)
	}

	targetVersionObj, err := c.newTargetVersionObjectFor(obj)
	if err != nil {
		return err
	}
	if err := c.converter.Convert(obj, targetVersionObj); err != nil {
		return errors.Wrapf(err, "failed to convert %s to target version while creating it", klog.KObj(obj))
	}

	if err := c.internalClient.Create(ctx, targetVersionObj, opts...); err != nil {
		return err
	}
	if err := c.converter.Convert(targetVersionObj, obj); err != nil {
		return errors.Wrapf(err, "failed to convert %s from target version while creating it", klog.KObj(targetVersionObj))
	}
	return nil
}

// Delete deletes the given obj from Kubernetes cluster.
func (c *conversionClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if !c.converter.IsConvertible(obj) {
		return c.internalClient.Delete(ctx, obj, opts...)
	}

	targetVersionObj, err := c.newTargetVersionObjectFor(obj)
	if err != nil {
		return err
	}
	if err := c.converter.Convert(obj, targetVersionObj); err != nil {
		return errors.Wrapf(err, "failed to convert %s to target version while deleting it", klog.KObj(obj))
	}

	if err := c.internalClient.Delete(ctx, targetVersionObj, opts...); err != nil {
		return err
	}

	if err := c.converter.Convert(targetVersionObj, obj); err != nil {
		return errors.Wrapf(err, "failed to convert %s from target version while deleting it", klog.KObj(targetVersionObj))
	}
	return nil
}

// Update updates the given obj in the Kubernetes cluster.
func (c *conversionClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if !c.converter.IsConvertible(obj) {
		return c.internalClient.Update(ctx, obj, opts...)
	}

	panic("update must not be used when conversion is required. Use patch instead")
}

// Patch patches the given obj in the Kubernetes cluster.
func (c *conversionClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if !c.converter.IsConvertible(obj) {
		return c.internalClient.Patch(ctx, obj, patch, opts...)
	}

	if _, ok := patch.(*conversionMergePatch); !ok {
		return errors.Errorf("patch must be created using sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/client.MergeFrom")
	}

	targetVersionObj, err := c.newTargetVersionObjectFor(obj)
	if err != nil {
		return err
	}
	if err := c.converter.Convert(obj, targetVersionObj); err != nil {
		return errors.Wrapf(err, "failed to convert %s to target version while patching it", klog.KObj(obj))
	}

	if err := c.internalClient.Patch(ctx, targetVersionObj, patch, opts...); err != nil {
		return err
	}
	if err := c.converter.Convert(targetVersionObj, obj); err != nil {
		return errors.Wrapf(err, "failed to convert %s from target version while patching it", klog.KObj(targetVersionObj))
	}
	return nil
}

// DeleteAllOf deletes all objects of the given type matching the given options.
func (c *conversionClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if !c.converter.IsConvertible(obj) {
		return c.internalClient.DeleteAllOf(ctx, obj, opts...)
	}

	// conversionClient only implements methods used in CAPV.
	panic("implement me")
}

func (c *conversionClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.internalClient.GroupVersionKindFor(obj)
}

func (c *conversionClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.internalClient.IsObjectNamespaced(obj)
}

func (c *conversionClient) Scheme() *runtime.Scheme {
	return c.internalClient.Scheme()
}

func (c *conversionClient) RESTMapper() meta.RESTMapper {
	return c.internalClient.RESTMapper()
}

func (c *conversionClient) Status() client.SubResourceWriter {
	return c.SubResource("status")
}

func (c *conversionClient) SubResource(subResource string) client.SubResourceClient {
	return &conversionSubResourceClient{conversionClient: c, subResource: subResource}
}

type conversionSubResourceClient struct {
	*conversionClient
	subResource string
}

// conversionClient must implement client.Client.
var _ client.SubResourceClient = &conversionSubResourceClient{}

func (c conversionSubResourceClient) Get(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceGetOption) error {
	// conversionSubResourceClient only implements methods used in CAPV.
	panic("implement me")
}

func (c conversionSubResourceClient) Create(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
	// conversionSubResourceClient only implements methods used in CAPV.
	panic("implement me")
}

func (c conversionSubResourceClient) Update(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
	panic("update must not be used when conversion is required. Use patch instead")
}

func (c conversionSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if !c.converter.IsConvertible(obj) {
		return c.internalClient.Status().Patch(ctx, obj, patch, opts...)
	}

	targetVersionObj, err := c.newTargetVersionObjectFor(obj)
	if err != nil {
		return err
	}
	if err := c.converter.Convert(obj, targetVersionObj); err != nil {
		return errors.Wrapf(err, "failed to convert %s to target version while patching status on it", klog.KObj(obj))
	}

	if err := c.internalClient.Status().Patch(ctx, targetVersionObj, patch, opts...); err != nil {
		return err
	}

	if err := c.converter.Convert(targetVersionObj, obj); err != nil {
		return errors.Wrapf(err, "failed to convert %s from target version while patching status on it", klog.KObj(targetVersionObj))
	}
	return nil
}

func (c conversionSubResourceClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.SubResourceApplyOption) error {
	if !c.converter.IsConvertible(obj.(runtime.Object)) {
		return c.internalClient.Status().Apply(ctx, obj, opts...)
	}

	// conversionSubResourceClient only implements methods used in CAPV.
	panic("implement me")
}

func (c *conversionClient) newTargetVersionObjectFor(obj client.Object) (client.Object, error) {
	gvk, err := c.converter.TargetGroupVersionKindFor(obj)
	if err != nil {
		return nil, err
	}

	o, err := c.internalClient.Scheme().New(gvk)
	if err != nil {
		return nil, err
	}

	targetObj, ok := o.(client.Object)
	if !ok {
		return nil, errors.Errorf("object for %s does not implement sigs.k8s.io/controller-runtime/pkg/client.Object", gvk)
	}
	return targetObj, nil
}

func (c *conversionClient) newTargetVersionObjectListFor(list client.ObjectList) (client.ObjectList, error) {
	gvk, err := c.converter.TargetGroupVersionKindFor(list)
	if err != nil {
		return nil, err
	}

	o, err := c.internalClient.Scheme().New(gvk)
	if err != nil {
		return nil, err
	}

	targetList, ok := o.(client.ObjectList)
	if !ok {
		return nil, errors.Errorf("object for %s does not implement sigs.k8s.io/controller-runtime/pkg/client.ObjectList", gvk)
	}
	return targetList, nil
}

func (c *conversionClient) newObjectListItemFor(list client.ObjectList) (client.Object, error) {
	gvkList, err := c.internalClient.GroupVersionKindFor(list)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(gvkList.Kind, "List") {
		return nil, errors.Errorf("object %s does not have a kind with the List suffix", gvkList)
	}

	gvk := schema.GroupVersionKind{
		Group:   gvkList.Group,
		Version: gvkList.Version,
		Kind:    strings.TrimSuffix(gvkList.Kind, "List"),
	}

	o, err := c.internalClient.Scheme().New(gvk)
	if err != nil {
		return nil, err
	}

	targetList, ok := o.(client.Object)
	if !ok {
		return nil, errors.Errorf("object for %s does not implement sigs.k8s.io/controller-runtime/pkg/client.Object", gvk)
	}
	return targetList, nil
}
