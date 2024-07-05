/*
Copyright 2024 The Kubernetes Authors.

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

// Package vcsim provide helpers for vcsim controller.
package vcsim

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

const vcsimInstanceName = "vcsim-e2e"

func Create(ctx context.Context, c client.Client) error {
	vcsim := &vcsimv1.VCenterSimulator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vcsimInstanceName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: vcsimv1.VCenterSimulatorSpec{},
	}

	Byf("Creating vcsim server %s", klog.KObj(vcsim))
	if err := c.Create(ctx, vcsim); err != nil {
		return err
	}

	if _, err := Get(ctx, c); err != nil {
		// Try best effort deletion of the unused VCenterSimulator before returning an error.
		_ = Delete(ctx, c, false)
		return err
	}
	return nil
}

func Get(ctx context.Context, c client.Client) (*vcsimv1.VCenterSimulator, error) {
	vcsim := &vcsimv1.VCenterSimulator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vcsimInstanceName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: vcsimv1.VCenterSimulatorSpec{},
	}

	var retryError error
	// Wait for the Server to report an address.
	_ = wait.PollUntilContextTimeout(ctx, time.Second, time.Second*5, true, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(vcsim), vcsim); err != nil {
			retryError = errors.Wrap(err, "getting VCenterSimulator")
			return false, nil
		}

		if vcsim.Status.Host == "" {
			retryError = errors.New("vcsim VCenterSimulator.Status.Host is not set")
			return false, nil
		}

		retryError = nil
		return true, nil
	})
	if retryError != nil {
		return nil, retryError
	}
	return vcsim, nil
}

func Delete(ctx context.Context, c client.Client, skipCleanup bool) error {
	if CurrentSpecReport().Failed() {
		By("Skipping cleanup of VCenterSimulator because the tests failed and the instance could still be in use")
		return nil
	}

	if skipCleanup {
		By("Skipping cleanup of VCenterSimulator because skipCleanup is set to true")
		return nil
	}

	vcsim := &vcsimv1.VCenterSimulator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vcsimInstanceName,
			Namespace: metav1.NamespaceDefault,
		},
	}
	Byf("Deleting VCenterSimulator %s", klog.KObj(vcsim))
	if err := c.Delete(ctx, vcsim); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
