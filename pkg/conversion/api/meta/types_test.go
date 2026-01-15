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

package meta

import (
	"testing"

	. "github.com/onsi/gomega"
)

type A struct {
	Source SourceTypeMeta
}

type B struct {
	Source string
}

type C struct{}

func Test_HasSource(t *testing.T) {
	tests := []struct {
		name string
		obj  any
		want bool
	}{
		{
			name: "Object with SourceTypeMeta",
			obj:  &A{},
			want: true,
		},
		{
			name: "Object with Source field but wrong type",
			obj:  &B{},
			want: false,
		},
		{
			name: "Object without SourceTypeMeta",
			obj:  &C{},
			want: false,
		},
		{
			name: "nil",
			obj:  nil,
			want: false,
		},
		{
			name: "not a pointer to struct",
			obj:  A{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(HasSource(tt.obj)).To(Equal(tt.want))
		})
	}
}

func Test_GetSource(t *testing.T) {
	tests := []struct {
		name       string
		obj        any
		wantSource SourceTypeMeta
		wantErr    bool
	}{
		{
			name:       "Object with SourceTypeMeta",
			obj:        &A{Source: SourceTypeMeta{APIVersion: "foo/bar"}},
			wantSource: SourceTypeMeta{APIVersion: "foo/bar"},
		},
		{
			name:    "Object with Source field but wrong type",
			obj:     &B{},
			wantErr: true,
		},
		{
			name:    "Object without SourceTypeMeta",
			obj:     &C{},
			wantErr: true,
		},
		{
			name:    "nil",
			obj:     nil,
			wantErr: true,
		},
		{
			name:    "not a pointer to struct",
			obj:     A{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, err := GetSource(tt.obj)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(got).To(BeNil())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).ToNot(BeNil())
			g.Expect(*got).To(Equal(tt.wantSource))
		})
	}
}

func Test_SetSource(t *testing.T) {
	tests := []struct {
		name    string
		obj     any
		Source  SourceTypeMeta
		wantErr bool
	}{
		{
			name:   "Object with SourceTypeMeta",
			obj:    &A{},
			Source: SourceTypeMeta{APIVersion: "foo/bar"},
		},
		{
			name:    "Object with Source field but wrong type",
			obj:     &B{},
			Source:  SourceTypeMeta{APIVersion: "foo/bar"},
			wantErr: true,
		},
		{
			name:    "Object without SourceTypeMeta",
			obj:     &C{},
			Source:  SourceTypeMeta{APIVersion: "foo/bar"},
			wantErr: true,
		},
		{
			name:    "nil",
			obj:     nil,
			Source:  SourceTypeMeta{APIVersion: "foo/bar"},
			wantErr: true,
		},
		{
			name:    "not a pointer to struct",
			obj:     A{},
			Source:  SourceTypeMeta{APIVersion: "foo/bar"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := SetSource(tt.obj, tt.Source)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			got, err := GetSource(tt.obj)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).ToNot(BeNil())
			g.Expect(*got).To(Equal(tt.Source))
		})
	}
}
