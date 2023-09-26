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

// Package fake implements a fake VMService for testing.
package fake

import (
	"context"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

type VMService struct {
	mock.Mock
}

func (v *VMService) ReconcileVM(_ context.Context, vmCtx *capvcontext.VMContext) (infrav1.VirtualMachine, error) {
	args := v.Called(vmCtx)
	return args.Get(0).(infrav1.VirtualMachine), args.Error(1)
}

func (v *VMService) DestroyVM(_ context.Context, vmCtx *capvcontext.VMContext) (reconcile.Result, infrav1.VirtualMachine, error) {
	args := v.Called(vmCtx)
	return args.Get(0).(reconcile.Result), args.Get(1).(infrav1.VirtualMachine), args.Error(2)
}
