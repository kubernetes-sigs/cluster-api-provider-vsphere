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

package govmomi

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
)

func Test_ShouldRetryTask(t *testing.T) {
	ctx := context.Background()

	t.Run("when no task is present", func(t *testing.T) {
		g := NewWithT(t)
		vmCtx := &capvcontext.VMContext{
			VSphereVM: &infrav1.VSphereVM{Status: infrav1.VSphereVMStatus{TaskRef: ""}},
		}
		reconciled, err := checkAndRetryTask(ctx, vmCtx, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(reconciled).To(BeFalse())
		g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
	})

	t.Run("when failed task was previously checked & RetryAfter time has not yet passed", func(t *testing.T) {
		g := NewWithT(t)
		vmCtx := &capvcontext.VMContext{
			VSphereVM: &infrav1.VSphereVM{Status: infrav1.VSphereVMStatus{
				TaskRef:    "task-123",
				RetryAfter: metav1.Time{Time: time.Now().Add(1 * time.Minute)},
			}},
		}

		// passing nil task since the task will not be reconciled
		reconciled, err := checkAndRetryTask(ctx, vmCtx, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(reconciled).To(BeFalse())
		g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
	})

	t.Run("when failed task was previously checked & RetryAfter time has passed", func(t *testing.T) {
		g := NewWithT(t)

		vmCtx := &capvcontext.VMContext{
			VSphereVM: &infrav1.VSphereVM{Status: infrav1.VSphereVMStatus{
				TaskRef:    "task-123",
				RetryAfter: metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
			}},
		}

		t.Run("for non error states", func(t *testing.T) {
			tests := []struct {
				task       mo.Task
				isRefEmpty bool
			}{
				{baseTask(types.TaskInfoStateQueued, ""), false},
				{baseTask(types.TaskInfoStateRunning, ""), false},
				{baseTask(types.TaskInfoStateSuccess, ""), true},
			}
			for i := range tests {
				tt := tests[i]
				t.Run(fmt.Sprintf("state: %s", tt.task.Info.State), func(t *testing.T) {
					g = NewWithT(t)
					reconciled, err := checkAndRetryTask(ctx, vmCtx, &tt.task)
					g.Expect(err).NotTo(HaveOccurred())
					if tt.isRefEmpty {
						g.Expect(reconciled).To(BeFalse())
						g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
					} else {
						g.Expect(reconciled).To(BeTrue())
						g.Expect(vmCtx.VSphereVM.Status.TaskRef).NotTo(BeEmpty())
					}
				})
			}
		})

		t.Run("for task in error state", func(t *testing.T) {
			task := baseTask(types.TaskInfoStateError, "task is stuck")

			reconciled, err := checkAndRetryTask(ctx, vmCtx, &task)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(reconciled).To(BeTrue())
			g.Expect(conditions.IsFalse(vmCtx.VSphereVM, infrav1.VMProvisionedCondition)).To(BeTrue())
			g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
			g.Expect(vmCtx.VSphereVM.Status.RetryAfter.IsZero()).To(BeTrue())
		})
	})

	t.Run("when failed task was previously not checked", func(t *testing.T) {
		g := NewWithT(t)
		vmCtx := &capvcontext.VMContext{
			VSphereVM: &infrav1.VSphereVM{Status: infrav1.VSphereVMStatus{
				// RetryAfter is not set since this is the first reconcile
				TaskRef: "task-123",
			}},
		}
		task := baseTask(types.TaskInfoStateError, "task is stuck")

		reconciled, err := checkAndRetryTask(ctx, vmCtx, &task)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(reconciled).To(BeTrue())
		g.Expect(conditions.IsFalse(vmCtx.VSphereVM, infrav1.VMProvisionedCondition)).To(BeTrue())
		g.Expect(vmCtx.VSphereVM.Status.RetryAfter.Unix()).To(BeNumerically("<=", metav1.Now().Add(1*time.Minute).Unix()))
	})
}

func baseTask(state types.TaskInfoState, errorDescription string) mo.Task {
	t := mo.Task{
		ExtensibleManagedObject: mo.ExtensibleManagedObject{
			Self: types.ManagedObjectReference{
				Value: "-for-logger",
			},
		},
	}
	if state != "" {
		t.Info = types.TaskInfo{
			State: state,
		}
	}
	if errorDescription != "" {
		t.Info.Description = &types.LocalizableMessage{
			Message: errorDescription,
		}
	}
	return t
}
