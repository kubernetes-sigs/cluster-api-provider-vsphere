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

// Package fake implements a fake ClusterModuleService for testing.
package fake

import (
	"context"

	"github.com/stretchr/testify/mock"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/clustermodule"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

type CMService struct {
	mock.Mock
}

func (f *CMService) Create(ctx context.Context, clusterCtx *capvcontext.ClusterContext, wrapper clustermodule.Wrapper) (string, error) {
	args := f.Called(ctx, clusterCtx, wrapper)
	return args.String(0), args.Error(1)
}

func (f *CMService) DoesExist(ctx context.Context, clusterCtx *capvcontext.ClusterContext, wrapper clustermodule.Wrapper, moduleUUID string) (bool, error) {
	args := f.Called(ctx, clusterCtx, wrapper, moduleUUID)
	return args.Bool(0), args.Error(1)
}

func (f *CMService) Remove(ctx context.Context, clusterCtx *capvcontext.ClusterContext, moduleUUID string) error {
	args := f.Called(ctx, clusterCtx, moduleUUID)
	return args.Error(0)
}
