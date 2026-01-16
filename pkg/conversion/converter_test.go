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
	"context"
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	conversionmeta "sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/api/meta"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/internal/test/api/hub"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/conversion/internal/test/api/v1alpha5"
)

// Note: test API types are defined in pkg/conversion/internal/test/api, however some types
// are intentionally been defined in this file e.g. to test when same type is already registered.

var (
	hubGroupVersion = schema.GroupVersion{Group: "vmoperator.vmware.com", Version: "hub"}

	hubConverterBuilder = NewConverterBuilder(hubGroupVersion, addConvertibleTypes)

	addHubToConverter = hubConverterBuilder.AddToConverter

	hubObjectTypes = []runtime.Object{}
)

func addConvertibleTypes(converter *Converter) error {
	return converter.AddTypes(hubGroupVersion, hubObjectTypes...)
}

func init() {
	hubObjectTypes = append(hubObjectTypes, &hub.A{}, &hub.AList{})
}

var (
	v1alpha5GroupVersion = schema.GroupVersion{Group: "vmoperator.vmware.com", Version: "v1alpha5"}

	v1alpha5ConverterBuilder = NewConverterBuilder(v1alpha5GroupVersion)

	AddV1alpha5ToConverter = v1alpha5ConverterBuilder.AddToConverter
)

var (
	v1alpha2GroupVersion = schema.GroupVersion{Group: "vmoperator.vmware.com", Version: "v1alpha2"}
)

func init() {
	v1alpha5ConverterBuilder.AddConversion(
		NewAddConversionBuilder(v1alpha5.ConvertAFromHubToV1alpha5, v1alpha5.ConvertAFromV1alpha5ToHub),
	)
}

type A struct {
	Foo string

	Source conversionmeta.SourceTypeMeta
}

func (in A) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

func (in A) DeepCopyObject() runtime.Object {
	panic("implement me")
}

// GetSource returns the Source for this object.
func (in *A) GetSource() conversionmeta.SourceTypeMeta {
	return in.Source
}

// SetSource sets Source for an API object.
func (in *A) SetSource(source conversionmeta.SourceTypeMeta) {
	in.Source = source
}

type B struct {
}

func (in B) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

func (in B) DeepCopyObject() runtime.Object {
	panic("implement me")
}

type BList struct{}

func (in BList) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

func (in BList) DeepCopyObject() runtime.Object {
	panic("implement me")
}

func Test_converter_AddTypes(t *testing.T) {
	tests := []struct {
		name    string
		gv      schema.GroupVersion
		obj     runtime.Object
		wantGvk schema.GroupVersionKind
		wantErr bool
	}{
		{
			name:    "Add type",
			gv:      hubGroupVersion,
			obj:     &hub.A{},
			wantGvk: hubGroupVersion.WithKind("A"),
		},
		{
			name:    "Add list type",
			gv:      hubGroupVersion,
			obj:     &hub.AList{},
			wantGvk: hubGroupVersion.WithKind("AList"),
		},
		{
			name:    "Fails for types which are not Convertible",
			gv:      hubGroupVersion,
			obj:     &B{},
			wantErr: true,
		},
		{
			name:    "Fails for empty group",
			gv:      schema.GroupVersion{Group: "", Version: "foo"},
			obj:     &hub.A{},
			wantErr: true,
		},
		{
			name:    "Fails for empty version",
			gv:      schema.GroupVersion{Group: "foo", Version: ""},
			obj:     &hub.A{},
			wantErr: true,
		},
		{
			name:    "Fails for nil object",
			gv:      hubGroupVersion,
			obj:     nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := NewConverter()

			err := c.AddTypes(tt.gv, tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(c.gvkToType).ToNot(HaveKey(tt.wantGvk))
				g.Expect(c.gvkConvertibleTypes).ToNot(HaveKey(tt.wantGvk))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(c.gvkToType).To(HaveKey(tt.wantGvk))
			T := c.gvkToType[tt.wantGvk]
			g.Expect(c.typeToGVK).To(HaveKeyWithValue(T, tt.wantGvk))
			g.Expect(c.gvkConvertibleTypes).To(HaveKeyWithValue(tt.wantGvk, true))
		})
	}
	t.Run("Pass when the same gvk/type is registered twice", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())
	})
	t.Run("Fails when a gvk is registered twice for different types", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddTypes(hubGroupVersion, &A{})
		g.Expect(err).To(HaveOccurred())
	})
	t.Run("Fails when a type is registered twice for different gvk", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddTypes(v1alpha5GroupVersion, &hub.A{})
		g.Expect(err).To(HaveOccurred())
	})
}

func TestConverter_AddConversion(t *testing.T) {
	c := NewConverter()
	utilruntime.Must(addHubToConverter(c))

	convertAFromHubToV1alpha5 := func(ctx context.Context, src runtime.Object, dst runtime.Object) error {
		return v1alpha5.ConvertAFromHubToV1alpha5(ctx, src.(*hub.A), dst.(*v1alpha5.A))
	}
	convertAFromV1alpha5ToHub := func(ctx context.Context, src runtime.Object, dst runtime.Object) error {
		return v1alpha5.ConvertAFromV1alpha5ToHub(ctx, src.(*v1alpha5.A), dst.(*hub.A))
	}
	tests := []struct {
		name       string
		gvSrc      schema.GroupVersion
		src        runtime.Object
		gvDst      schema.GroupVersion
		dst        runtime.Object
		srcToDst   ConvertFunc
		dstToSrc   ConvertFunc
		wantSrcGvk schema.GroupVersionKind
		wantDstGvk schema.GroupVersionKind
		wantErr    bool
	}{
		{
			name:       "Add conversion",
			gvSrc:      hubGroupVersion,
			src:        &hub.A{},
			gvDst:      v1alpha5GroupVersion,
			dst:        &v1alpha5.A{},
			srcToDst:   convertAFromHubToV1alpha5,
			dstToSrc:   convertAFromV1alpha5ToHub,
			wantSrcGvk: hubGroupVersion.WithKind("A"),
			wantDstGvk: v1alpha5GroupVersion.WithKind("A"),
		},
		{
			name:    "Fails for empty group",
			gvSrc:   hubGroupVersion,
			src:     &hub.A{},
			gvDst:   schema.GroupVersion{Group: "", Version: "foo"},
			dst:     &v1alpha5.A{},
			wantErr: true,
		},
		{
			name:    "Fails for empty version",
			gvSrc:   hubGroupVersion,
			src:     &hub.A{},
			gvDst:   schema.GroupVersion{Group: "foo", Version: ""},
			dst:     &v1alpha5.A{},
			wantErr: true,
		},
		{
			name:    "Fails when source kind has List suffix",
			gvSrc:   hubGroupVersion,
			src:     &hub.AList{},
			gvDst:   v1alpha5GroupVersion,
			dst:     &BList{},
			wantErr: true,
		},
		{
			name:    "Fails for target version equal o source version",
			gvSrc:   hubGroupVersion,
			src:     &hub.A{},
			gvDst:   hubGroupVersion,
			dst:     &hub.A{},
			wantErr: true,
		},
		{
			name:    "Fails when source and target kind differ",
			gvSrc:   hubGroupVersion,
			src:     &hub.A{},
			gvDst:   v1alpha5GroupVersion,
			dst:     &B{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			err := c.AddConversion(tt.src, tt.gvDst.Version, tt.dst, tt.srcToDst, tt.dstToSrc)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(c.gvkToType).ToNot(HaveKey(tt.wantDstGvk))
				g.Expect(c.gvkConvertibleTypes).ToNot(HaveKey(tt.wantDstGvk))
				g.Expect(c.conversionFuncs).ToNot(HaveKey(tt.wantSrcGvk))
				g.Expect(c.conversionFuncs).ToNot(HaveKey(tt.wantDstGvk))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(c.gvkToType).To(HaveKey(tt.wantDstGvk))
			T := c.gvkToType[tt.wantDstGvk]
			g.Expect(c.typeToGVK).To(HaveKeyWithValue(T, tt.wantDstGvk))
			g.Expect(c.gvkConvertibleTypes).To(HaveKeyWithValue(tt.wantDstGvk, false))
			g.Expect(c.conversionFuncs).To(HaveKey(tt.wantSrcGvk))
			g.Expect(c.conversionFuncs[tt.wantSrcGvk]).To(HaveKey(tt.wantDstGvk))
			g.Expect(c.conversionFuncs).To(HaveKey(tt.wantDstGvk))
			g.Expect(c.conversionFuncs[tt.wantDstGvk]).To(HaveKey(tt.wantSrcGvk))
		})
	}
	t.Run("Pass when the same conversion is registered twice", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).ToNot(HaveOccurred())
	})
	t.Run("Fails when a gvk is registered twice for different types", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).To(HaveOccurred())
	})
	t.Run("Fails when a type is registered twice for different gvk", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha2GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).To(HaveOccurred())
	})
	t.Run("Fails when the same conversion is registered twice but with a different conversion func", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, func(_ context.Context, _ runtime.Object, _ runtime.Object) error { return nil }, convertAFromV1alpha5ToHub)
		g.Expect(err).To(HaveOccurred())
	})
	t.Run("Fails when the same conversion is registered twice but with a different conversion func", func(t *testing.T) {
		g := NewWithT(t)

		c := NewConverter()

		err := c.AddTypes(hubGroupVersion, &hub.A{})
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, convertAFromV1alpha5ToHub)
		g.Expect(err).ToNot(HaveOccurred())

		err = c.AddConversion(&hub.A{}, v1alpha5GroupVersion.Version, &v1alpha5.A{}, convertAFromHubToV1alpha5, func(_ context.Context, _ runtime.Object, _ runtime.Object) error { return nil })
		g.Expect(err).To(HaveOccurred())
	})
}

func Test_converter_Convert(t *testing.T) {
	c := NewConverter()
	utilruntime.Must(addHubToConverter(c))
	utilruntime.Must(AddV1alpha5ToConverter(c))

	tests := []struct {
		name      string
		converter *Converter
		src       runtime.Object
		dst       runtime.Object
		wantDst   runtime.Object
		wantErr   bool
	}{
		{
			name:      "Convert from convertible object to another version",
			converter: c,
			src:       &hub.A{Foo: "bar"},
			dst:       &v1alpha5.A{},
			wantDst:   &v1alpha5.A{Foo: "bar"},
		},
		{
			name:      "Convert from another version to convertible object",
			converter: c,
			src:       &v1alpha5.A{Foo: "bar"},
			dst:       &hub.A{},
			wantDst: &hub.A{
				Foo: "bar",
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: v1alpha5GroupVersion.String(),
				},
			},
		},
		{
			name:      "Fails when convertible object has a different source version than the target version",
			converter: c,
			src: &hub.A{
				Foo: "bar",
				Source: conversionmeta.SourceTypeMeta{
					APIVersion: v1alpha2GroupVersion.String(),
				},
			},
			dst:     &v1alpha5.A{},
			wantErr: true,
		},
		{
			name:      "Fails when conversion is not defined",
			converter: c,
			src:       &hub.A{},
			dst:       &B{},
			wantErr:   true,
		},
		{
			name: "Fails when conversion fail",
			converter: func() *Converter {
				c := NewConverter()
				c.SetTargetVersion(v1alpha5GroupVersion.Version)
				utilruntime.Must(addHubToConverter(c))
				_ = c.AddConversion(
					&hub.A{},
					v1alpha5GroupVersion.Version, &v1alpha5.A{},
					func(_ context.Context, _ runtime.Object, _ runtime.Object) error {
						return errors.New("fail")
					}, func(_ context.Context, _ runtime.Object, _ runtime.Object) error {
						return nil
					},
				)
				return c
			}(),
			src:     &hub.A{},
			dst:     &v1alpha5.A{},
			wantErr: true,
		},
		{
			name:      "Fails for nil object",
			converter: c,
			src:       nil,
			dst:       &hub.A{},
			wantErr:   true,
		},
		{
			name:      "Fails for nil object",
			converter: c,
			src:       &hub.A{},
			dst:       nil,
			wantErr:   true,
		},
		{
			name:      "Fails for unknown object",
			converter: c,
			src:       &hub.A{},
			dst:       &corev1.Node{},
			wantErr:   true,
		},
		{
			name:      "Fails for unknown object",
			converter: c,
			src:       &corev1.Node{},
			dst:       &hub.A{},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			err := tt.converter.Convert(context.TODO(), tt.src, tt.dst)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(tt.dst).To(Equal(tt.wantDst))
		})
	}
}

func Test_converter_IsConvertible(t *testing.T) {
	c := NewConverter()
	utilruntime.Must(addHubToConverter(c))
	utilruntime.Must(AddV1alpha5ToConverter(c))

	tests := []struct {
		name string
		obj  runtime.Object
		want bool
	}{
		{
			name: "Return true for a convertible object",
			obj:  &hub.A{},
			want: true,
		},
		{
			name: "Return false for a type a convertible object can convert to",
			obj:  &v1alpha5.A{},
			want: false,
		},
		{
			name: "Return false for nil type",
			obj:  nil,
			want: false,
		},
		{
			name: "Return false for unknown type",
			obj:  &corev1.Node{},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := c.IsConvertible(tt.obj)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func Test_converter_TargetGroupVersionKindFor(t *testing.T) {
	c := NewConverter()
	c.SetTargetVersion(v1alpha5GroupVersion.Version)
	utilruntime.Must(addHubToConverter(c))
	utilruntime.Must(AddV1alpha5ToConverter(c))

	tests := []struct {
		name      string
		converter *Converter
		obj       runtime.Object
		wantGvk   schema.GroupVersionKind
		wantErr   bool
	}{
		{
			name:      "Get gvk for convertible object",
			converter: c,
			obj:       &hub.A{},
			wantGvk:   v1alpha5GroupVersion.WithKind("A"),
		},
		{
			name:      "Get gvk for convertible objectList",
			converter: c,
			obj:       &hub.AList{},
			wantGvk:   v1alpha5GroupVersion.WithKind("AList"),
		},
		{
			name: "Fails when conversions to the target version are not registered",
			converter: func() *Converter {
				c := NewConverter()
				c.SetTargetVersion(v1alpha5GroupVersion.Version)
				utilruntime.Must(addHubToConverter(c))
				return c
			}(),
			obj:     &hub.A{},
			wantErr: true,
		},
		{
			name:      "Fails for a type a convertible object can convert to",
			converter: c,
			obj:       &v1alpha5.A{},
			wantErr:   true,
		},
		{
			name:      "Fails for nil type",
			converter: c,
			obj:       nil,
			wantErr:   true,
		},
		{
			name:      "Fails for unknown type",
			converter: c,
			obj:       &corev1.Node{},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			gvk, err := tt.converter.TargetGroupVersionKindFor(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(gvk).To(Equal(schema.GroupVersionKind{}))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gvk).To(Equal(tt.wantGvk))
		})
	}
}

func Test_converter_GroupVersionKindFor(t *testing.T) {
	tests := []struct {
		name      string
		converter *Converter
		obj       runtime.Object
		wantGvk   schema.GroupVersionKind
		wantErr   bool
	}{
		{
			name: "Get gvk for convertible object",
			converter: func() *Converter {
				c := NewConverter()
				utilruntime.Must(addHubToConverter(c))
				return c
			}(),
			obj:     &hub.A{},
			wantGvk: hubGroupVersion.WithKind("A"),
		},
		{
			name: "Get gvk for a type a convertible object can convert to",
			converter: func() *Converter {
				c := NewConverter()
				c.SetTargetVersion(v1alpha5GroupVersion.Version)
				utilruntime.Must(addHubToConverter(c))
				utilruntime.Must(AddV1alpha5ToConverter(c))
				return c
			}(),
			obj:     &v1alpha5.A{},
			wantGvk: v1alpha5GroupVersion.WithKind("A"),
		},
		{
			name: "Fails for nil type",
			converter: func() *Converter {
				c := NewConverter()
				utilruntime.Must(addHubToConverter(c))
				return c
			}(),
			obj:     nil,
			wantErr: true,
		},
		{
			name: "Fails for unknown type",
			converter: func() *Converter {
				c := NewConverter()
				utilruntime.Must(addHubToConverter(c))
				return c
			}(),
			obj:     &corev1.Node{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			gvk, err := tt.converter.GroupVersionKindFor(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(gvk).To(Equal(schema.GroupVersionKind{}))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gvk).To(Equal(tt.wantGvk))
		})
	}
}

func Test_objType(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Object
		wantT   reflect.Type
		wantErr bool
	}{
		{
			name:  "return type",
			obj:   &hub.A{},
			wantT: reflect.TypeOf(hub.A{}),
		},
		{
			name:    "Fails nil object",
			obj:     nil,
			wantErr: true,
		},
		{
			name:    "Fails non pointer object",
			obj:     hub.A{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := NewConverter()

			utilruntime.Must(addHubToConverter(c))

			gotT, err := objType(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotT).To(Equal(tt.wantT))
		})
	}
}
