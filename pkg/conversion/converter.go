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
func (s *Converter) AddConversion(src runtime.Object, version string, dst runtime.Object, srcToDst, dstToSrc any) error {
	gvkSrc, err := s.GroupVersionKindFor(src)
	if err != nil {
		return err
	}
	tSrc := s.gvkToType[gvkSrc]

	if strings.HasSuffix(gvkSrc.Kind, "List") {
		return errors.New("invalid source type, source type for a conversion cannot have the List suffix")
	}

	tDst, err := objType(dst)
	if err != nil {
		return err
	}

	gvkDst := schema.GroupVersionKind{
		Group:   gvkSrc.Group,
		Version: version,
		Kind:    tDst.Name(),
	}
	if gvkDst.Version == "" {
		return errors.New("invalid version, version cannot be empty")
	}

	if gvkDst.Version == gvkSrc.Version {
		return errors.New("invalid version, target version for a conversion cannot be the same registered for the source object")
	}

	if gvkSrc.Kind != gvkDst.Kind {
		return errors.New("invalid destination type, destination type for a conversion must be of the same kind of the source object")
	}

	if oldT, found := s.gvkToType[gvkDst]; found && oldT != tDst {
		return errors.Errorf("double registration of different types for %v: old=%v.%v, new=%v.%v", gvkDst, oldT.PkgPath(), oldT.Name(), tDst.PkgPath(), tDst.Name())
	}

	if oldGvk, found := s.typeToGVK[tDst]; found && oldGvk != gvkDst {
		return errors.Errorf("double registration of different gvk for %v.%v: old=%s, new=%s", tDst.PkgPath(), tDst.Name(), oldGvk, gvkDst)
	}

	if err := conversionFuncIsValid(tSrc, tDst, srcToDst); err != nil {
		return errors.Wrapf(err, "invalid conversion function from %v to %v", gvkSrc, gvkDst.Version)
	}

	if err := conversionFuncIsValid(tDst, tSrc, dstToSrc); err != nil {
		return errors.Wrapf(err, "invalid conversion function from %v to %v", gvkDst, gvkSrc.Version)
	}

	s.gvkToType[gvkDst] = tDst
	s.gvkConvertibleTypes[gvkDst] = false
	s.typeToGVK[tDst] = gvkDst

	if s.conversionFuncs[gvkSrc] == nil {
		s.conversionFuncs[gvkSrc] = map[schema.GroupVersionKind]reflect.Value{}
	}

	srcToDstV := reflect.ValueOf(srcToDst)
	if oldC, found := s.conversionFuncs[gvkSrc][gvkDst]; found && oldC != srcToDstV {
		return errors.Errorf("double registration of conversion function from %v to %v: old function is different from the new function", gvkSrc, gvkDst.Version)
	}
	s.conversionFuncs[gvkSrc][gvkDst] = srcToDstV

	dstToSrcV := reflect.ValueOf(dstToSrc)
	if s.conversionFuncs[gvkDst] == nil {
		s.conversionFuncs[gvkDst] = map[schema.GroupVersionKind]reflect.Value{}
	}
	if oldC, found := s.conversionFuncs[gvkDst][gvkSrc]; found && oldC != dstToSrcV {
		return errors.Errorf("double registration of conversion function from %v to %v: old function is different from the new function", gvkDst, gvkSrc.Version)
	}
	s.conversionFuncs[gvkDst][gvkSrc] = dstToSrcV

	return nil
}

// Convert converts an object into another with the same kind, but a different version.
func (s *Converter) Convert(src runtime.Object, dst runtime.Object) error {
	gvkSrc, err := s.GroupVersionKindFor(src)
	if err != nil {
		return err
	}

	gvkDst, err := s.GroupVersionKindFor(dst)
	if err != nil {
		return err
	}

	if s.gvkConvertibleTypes[gvkSrc] {
		source, err := conversionmeta.GetSource(src)
		if err != nil {
			return err
		}

		if source.APIVersion != "" && source.APIVersion != gvkDst.GroupVersion().String() {
			return errors.Errorf("objects with kind %s and source.APIVersion %s cannot be converted to %s", gvkSrc.Kind, source.APIVersion, gvkDst.Version)
		}
	}

	conversionFunc, ok := s.conversionFuncs[gvkSrc][gvkDst]
	if !ok {
		return errors.Errorf("no conversion registered from %s to %s", gvkSrc, gvkDst)
	}

	args := []reflect.Value{
		reflect.ValueOf(src),
		reflect.ValueOf(dst),
	}

	results := conversionFunc.Call(args)

	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}

	if s.gvkConvertibleTypes[gvkDst] {
		if err := conversionmeta.SetSource(dst, conversionmeta.SourceTypeMeta{APIVersion: gvkSrc.GroupVersion().String()}); err != nil {
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
