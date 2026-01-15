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

package conversion

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	conversionmeta "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta"
)

// Converter defines methods for converting API objects.
type Converter struct {
	// gvkToType allows to figure out the go type for an object with the given gkv.
	gvkToType map[schema.GroupVersionKind]reflect.Type

	// typeToGVK allows to find the gkv for a given go object.
	typeToGVK map[reflect.Type]schema.GroupVersionKind

	// gvkConvertibleTypes allows to figure out if an object is a convertible type or not.
	gvkConvertibleTypes map[schema.GroupVersionKind]bool

	// conversionFuncs stores func to convert objects with a given gvk to another.
	conversionFuncs map[schema.GroupVersionKind]map[schema.GroupVersionKind]reflect.Value

	// targetVersionSelector stores func that selects the target version for conversions.
	targetVersionSelector func(gk schema.GroupKind) string
}

// NewConverter returns a Converter.
func NewConverter() *Converter {
	s := &Converter{
		gvkToType:           map[schema.GroupVersionKind]reflect.Type{},
		typeToGVK:           map[reflect.Type]schema.GroupVersionKind{},
		gvkConvertibleTypes: map[schema.GroupVersionKind]bool{},
		conversionFuncs:     map[schema.GroupVersionKind]map[schema.GroupVersionKind]reflect.Value{},
		targetVersionSelector: func(_ schema.GroupKind) string {
			panic("targetVersionSelector not set")
		},
	}
	return s
}

// SetTargetVersion sets the target version to be used for all groups and kinds known by this converter.
func (s *Converter) SetTargetVersion(v string) {
	s.targetVersionSelector = func(_ schema.GroupKind) string { return v }
}

// AddTypes adds to the converter types that require conversion.
func (s *Converter) AddTypes(gv schema.GroupVersion, types ...runtime.Object) error {
	if gv.Group == "" {
		return errors.Errorf("invalid group, group cannot be empty")
	}

	if gv.Version == "" {
		return errors.Errorf("invalid version, version cannot be empty")
	}

	for _, obj := range types {
		t, err := objType(obj)
		if err != nil {
			return err
		}

		if !strings.HasSuffix(t.Name(), "List") && !conversionmeta.HasSource(obj) {
			return errors.Errorf("all objects must have a source field of type sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta.SourceTypeMeta, %v.%v does not have this field", t.PkgPath(), t.Name())
		}

		gvk := gv.WithKind(t.Name())
		if oldT, found := s.gvkToType[gvk]; found && oldT != t {
			return errors.Errorf("double registration of different types for %v: old=%v.%v, new=%v.%v", gvk, oldT.PkgPath(), oldT.Name(), t.PkgPath(), t.Name())
		}

		if oldGvk, found := s.typeToGVK[t]; found && oldGvk != gvk {
			return errors.Errorf("double registration of different gvk for %v.%v: old=%s, new=%s", t.PkgPath(), t.Name(), oldGvk, gvk)
		}

		s.gvkToType[gvk] = t
		s.gvkConvertibleTypes[gvk] = true
		s.typeToGVK[t] = gvk
	}
	return nil
}

// AllKnownTypes returns the all known types.
// Note: only convertible types are included.
func (s *Converter) AllKnownTypes() map[schema.GroupVersionKind]reflect.Type {
	r := map[schema.GroupVersionKind]reflect.Type{}
	for gvk, isConvertible := range s.gvkConvertibleTypes {
		if isConvertible {
			r[gvk] = s.gvkToType[gvk]
		}
	}
	return r
}

// Recognizes returns true if the converter is able to handle the provided group,version,kind
// of an object.
func (s *Converter) Recognizes(gvk schema.GroupVersionKind) bool {
	_, exists := s.gvkToType[gvk]
	return exists
}

// AddConversion adds to the Converter functions to be used when converting objects from one version to another.
// For instance, adding conversion from vmoprhub.VirtualMachine to vmoprv1alpha2.VirtualMachine will look like
//
// converter.AddConversion(
//
//	&vmoprvhub.VirtualMachine{},
//	vmoprv1alpha2.GroupVersion.Version, &vmoprv1alpha2.VirtualMachine{},
//	convert_hub_VirtualMachine_To_v1alpha2_VirtualMachine, convert_v1alpha2_VirtualMachine_To_hub_VirtualMachine,
//
// )
//
// More examples can be found in pkg/conversion/api/vmoperator.
func (s *Converter) AddConversion(src runtime.Object, dstVersion string, dst runtime.Object, srcToDstFunc, dstToSrcFunc any) error {
	srcGVK, err := s.GroupVersionKindFor(src)
	if err != nil {
		return err
	}
	srcType := s.gvkToType[srcGVK]

	if strings.HasSuffix(srcGVK.Kind, "List") {
		return errors.New("invalid source type, source type for a conversion cannot have the List suffix")
	}

	dstType, err := objType(dst)
	if err != nil {
		return err
	}

	dstGVK := schema.GroupVersionKind{
		Group:   srcGVK.Group,
		Version: dstVersion,
		Kind:    dstType.Name(),
	}
	if dstGVK.Group == "" {
		return errors.Errorf("invalid group, group cannot be empty")
	}
	if dstGVK.Version == "" {
		return errors.New("invalid version, version cannot be empty")
	}

	if dstGVK.Version == srcGVK.Version {
		return errors.New("invalid version, target version for a conversion cannot be the equal to the version registered for the source object")
	}

	if srcGVK.Kind != dstGVK.Kind {
		return errors.New("invalid destination type, destination type for a conversion must be of the same kind as the source object")
	}

	if oldT, found := s.gvkToType[dstGVK]; found && oldT != dstType {
		return errors.Errorf("double registration of different types for %v: old=%v.%v, new=%v.%v", dstGVK, oldT.PkgPath(), oldT.Name(), dstType.PkgPath(), dstType.Name())
	}

	if oldGVK, found := s.typeToGVK[dstType]; found && oldGVK != dstGVK {
		return errors.Errorf("double registration of different gvk for %v.%v: old=%s, new=%s", dstType.PkgPath(), dstType.Name(), oldGVK, dstGVK)
	}

	if err := conversionFuncIsValid(srcType, dstType, srcToDstFunc); err != nil {
		return errors.Wrapf(err, "invalid conversion function from %v to %v", srcGVK, dstGVK.Version)
	}

	if err := conversionFuncIsValid(dstType, srcType, dstToSrcFunc); err != nil {
		return errors.Wrapf(err, "invalid conversion function from %v to %v", dstGVK, srcGVK.Version)
	}

	s.gvkToType[dstGVK] = dstType
	s.gvkConvertibleTypes[dstGVK] = false
	s.typeToGVK[dstType] = dstGVK

	if s.conversionFuncs[srcGVK] == nil {
		s.conversionFuncs[srcGVK] = map[schema.GroupVersionKind]reflect.Value{}
	}

	srcToDstFuncV := reflect.ValueOf(srcToDstFunc)
	if oldC, found := s.conversionFuncs[srcGVK][dstGVK]; found && oldC != srcToDstFuncV {
		return errors.Errorf("double registration of conversion function from %v to %v: old function is different from the new function", srcGVK, dstGVK.Version)
	}
	s.conversionFuncs[srcGVK][dstGVK] = srcToDstFuncV

	dstToSrcFuncV := reflect.ValueOf(dstToSrcFunc)
	if s.conversionFuncs[dstGVK] == nil {
		s.conversionFuncs[dstGVK] = map[schema.GroupVersionKind]reflect.Value{}
	}
	if oldC, found := s.conversionFuncs[dstGVK][srcGVK]; found && oldC != dstToSrcFuncV {
		return errors.Errorf("double registration of conversion function from %v to %v: old function is different from the new function", dstGVK, srcGVK.Version)
	}
	s.conversionFuncs[dstGVK][srcGVK] = dstToSrcFuncV

	return nil
}

// Convert converts an object into another with the same kind, but a different version.
func (s *Converter) Convert(src runtime.Object, dst runtime.Object) error {
	srcGVK, err := s.GroupVersionKindFor(src)
	if err != nil {
		return err
	}

	dstGVK, err := s.GroupVersionKindFor(dst)
	if err != nil {
		return err
	}

	if s.gvkConvertibleTypes[srcGVK] {
		source, err := conversionmeta.GetSource(src)
		if err != nil {
			return err
		}

		if source.APIVersion != "" && source.APIVersion != dstGVK.GroupVersion().String() {
			return errors.Errorf("objects with kind %s and source.APIVersion %s cannot be converted to %s", srcGVK.Kind, source.APIVersion, dstGVK.Version)
		}
	}

	conversionFunc, ok := s.conversionFuncs[srcGVK][dstGVK]
	if !ok {
		return errors.Errorf("no conversion registered from %s to %s", srcGVK, dstGVK)
	}

	args := []reflect.Value{
		reflect.ValueOf(src),
		reflect.ValueOf(dst),
	}

	results := conversionFunc.Call(args)

	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}

	if s.gvkConvertibleTypes[dstGVK] {
		if err := conversionmeta.SetSource(dst, conversionmeta.SourceTypeMeta{APIVersion: srcGVK.GroupVersion().String()}); err != nil {
			return err
		}
	}
	return nil
}

// IsConvertible return true if an object requires conversion before write and after read.
func (s *Converter) IsConvertible(obj runtime.Object) bool {
	gvk, err := s.GroupVersionKindFor(obj)
	if err != nil {
		return false
	}
	return s.gvkConvertibleTypes[gvk]
}

// TargetGroupVersionKindFor returns the GroupVersionKind for the given object converted to the target version.
// Note: This func should only be called with types that require conversion.
func (s *Converter) TargetGroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvk, err := s.GroupVersionKindFor(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	isList := false
	if strings.HasSuffix(gvk.Kind, "List") {
		isList = true
		gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
	}

	if !s.gvkConvertibleTypes[gvk] {
		return schema.GroupVersionKind{}, errors.Errorf("no type registered for %s", gvk)
	}

	targetGVK := schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: s.targetVersionSelector(gvk.GroupKind()),
		Kind:    gvk.Kind,
	}

	if _, ok := s.conversionFuncs[gvk][targetGVK]; !ok {
		return schema.GroupVersionKind{}, errors.Errorf("no conversion registered from %s to %s", gvk, targetGVK.Version)
	}

	if isList {
		targetGVK.Kind = fmt.Sprintf("%sList", targetGVK.Kind)
	}

	return targetGVK, nil
}

// GroupVersionKindFor returns the GroupVersionKind for the given object.
// Note: obj can be either a type that requires conversions or one of the types it can convert to/from.
func (s *Converter) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	t, err := objType(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	gvk, ok := s.typeToGVK[t]
	if !ok {
		return schema.GroupVersionKind{}, errors.Errorf("no type registered for %s.%s", t.PkgPath(), t.Name())
	}
	return gvk, nil
}

var errorType = reflect.TypeFor[error]()

// conversionFuncIsValid validates conversion func signature.
// A valid func signature takes in input source and destination type and return and error, func(src A, dst B) error.
func conversionFuncIsValid(tSrc, tDst reflect.Type, f any) error {
	errFor := func(msg string) error {
		return errors.Errorf("conversion func must be a func(%s.%s, %s.%s) error, %s", tSrc.PkgPath(), tSrc.Name(), tDst.PkgPath(), tDst.Name(), msg)
	}

	errForT := func(t reflect.Type) error {
		return errFor(fmt.Sprintf("got %v", t))
	}

	if f == nil {
		return errFor("got nil")
	}

	t := reflect.TypeOf(f)
	if t.Kind() != reflect.Func {
		return errForT(t)
	}

	if t.NumIn() != 2 {
		return errForT(t)
	}

	if t.In(0) != reflect.PointerTo(tSrc) {
		return errForT(t)
	}

	if t.In(1) != reflect.PointerTo(tDst) {
		return errForT(t)
	}

	if t.NumOut() != 1 {
		return errForT(t)
	}

	if !t.Out(0).Implements(errorType) {
		return errForT(t)
	}
	return nil
}

func objType(obj runtime.Object) (reflect.Type, error) {
	if obj == nil {
		return nil, errors.New("all objects must be pointers to structs, got nil")
	}

	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Pointer {
		return nil, errors.Errorf("all objects must be pointers to structs, got %s", t.Kind())
	}
	t = t.Elem()
	if t.Kind() != reflect.Struct {
		return nil, errors.Errorf("all objects must be pointers to structs, got *%s", t.Kind())
	}
	return t, nil
}
