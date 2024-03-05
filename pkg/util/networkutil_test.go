/*
Copyright 2023 The Kubernetes Authors.

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

package util

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNCPSupportFW(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	tests := []struct {
		name    string
		client  client.Client
		want    bool
		wantErr bool
	}{
		{
			"No version configmap",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build(),
			false,
			true,
		},
		{
			"non-semver version",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(newNCPConfigMap("nosemver")).Build(),
			false,
			true,
		},
		{
			"compatible version lower end",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(newNCPConfigMap(NCPVersionSupportFW)).Build(),
			true,
			false,
		},
		{
			"compatible version upper end",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(newNCPConfigMap("3.0.9999")).Build(),
			true,
			false,
		},
		{
			"compatible version with more than 3 segments",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(newNCPConfigMap("3.0.1.1.1")).Build(),
			true,
			false,
		},
		{
			"incompatible version lower end",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(newNCPConfigMap("3.0.0")).Build(),
			false,
			false,
		},
		{
			"incompatible version upper end",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(newNCPConfigMap(NCPVersionSupportFWEnded)).Build(),
			false,
			false,
		},
		{
			"incompatible version with more than 3 segments",
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(newNCPConfigMap("3.1.0.1")).Build(),
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := NCPSupportFW(ctx, tt.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("NCPSupportFW() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NCPSupportFW() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newNCPConfigMap(version string) client.Object {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NCPVersionConfigMap,
			Namespace: NCPNamespace,
		},
		Data: map[string]string{
			NCPVersionKey: version,
		},
	}
}
