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

package framework

import (
	"context"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// waitForDaemonSetAvailableInput is the input for waitForDaemonSetAvailable.
type waitForDaemonSetAvailableInput struct {
	Getter    framework.Getter
	Daemonset *appsv1.DaemonSet
}

// waitForDaemonSetAvailable waits until the DaemonSet is rolled out:
// * status.updatedNumberScheduled < status.DesiredNumberScheduled.
// * status.NumberAvailable < status.DesiredNumberScheduled.
func waitForDaemonSetAvailable(ctx context.Context, input waitForDaemonSetAvailableInput, intervals ...interface{}) {
	Byf("Waiting for daemonset %s to be available", klog.KObj(input.Daemonset))
	daemonSet := &appsv1.DaemonSet{}
	Eventually(func() bool {
		key := client.ObjectKey{
			Namespace: input.Daemonset.GetNamespace(),
			Name:      input.Daemonset.GetName(),
		}
		if err := input.Getter.Get(ctx, key, daemonSet); err != nil {
			return false
		}
		if daemonSet.Generation <= daemonSet.Status.ObservedGeneration {
			if daemonSet.Status.UpdatedNumberScheduled < daemonSet.Status.DesiredNumberScheduled {
				return false
			}
			if daemonSet.Status.NumberAvailable < daemonSet.Status.DesiredNumberScheduled {
				return false
			}
			return true
		}
		return false
	}, intervals...).Should(BeTrue())
}
