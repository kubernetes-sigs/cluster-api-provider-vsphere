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
	context2 "context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

func Test_ShouldRetryTask(t *testing.T) {
	t.Run("when no task is present", func(t *testing.T) {
		g := NewWithT(t)
		vmCtx := &context.VMContext{
			VSphereVM: &infrav1.VSphereVM{Status: infrav1.VSphereVMStatus{TaskRef: ""}},
		}

		reconciled, err := checkAndRetryTask(vmCtx, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(reconciled).To(BeFalse())
		g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
	})

	t.Run("when failed task was previously checked & RetryAfter time has not yet passed", func(t *testing.T) {
		g := NewWithT(t)
		vmCtx := &context.VMContext{
			VSphereVM: &infrav1.VSphereVM{Status: infrav1.VSphereVMStatus{
				TaskRef:    "task-123",
				RetryAfter: metav1.Time{Time: time.Now().Add(1 * time.Minute)},
			}},
		}

		// passing nil task since the task will not be reconciled
		reconciled, err := checkAndRetryTask(vmCtx, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(reconciled).To(BeFalse())
		g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
	})

	t.Run("when failed task was previously checked & RetryAfter time has passed", func(t *testing.T) {
		g := NewWithT(t)

		vmCtx := &context.VMContext{
			Logger: logr.Discard(),
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
			for _, tt := range tests {
				t.Run(fmt.Sprintf("state: %s", tt.task.Info.State), func(t *testing.T) {
					g = NewWithT(t)
					reconciled, err := checkAndRetryTask(vmCtx, &tt.task)
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

			reconciled, err := checkAndRetryTask(vmCtx, &task)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(reconciled).To(BeTrue())
			g.Expect(conditions.IsFalse(vmCtx.VSphereVM, infrav1.VMProvisionedCondition))
			g.Expect(vmCtx.VSphereVM.Status.TaskRef).To(BeEmpty())
			g.Expect(vmCtx.VSphereVM.Status.RetryAfter.IsZero()).To(BeTrue())
		})
	})

	t.Run("when failed task was previously not checked", func(t *testing.T) {
		g := NewWithT(t)
		vmCtx := &context.VMContext{
			Logger: logr.Discard(),
			VSphereVM: &infrav1.VSphereVM{Status: infrav1.VSphereVMStatus{
				// RetryAfter is not set since this is the first reconcile
				TaskRef: "task-123",
			}},
		}
		task := baseTask(types.TaskInfoStateError, "task is stuck")

		reconciled, err := checkAndRetryTask(vmCtx, &task)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(reconciled).To(BeTrue())
		g.Expect(conditions.IsFalse(vmCtx.VSphereVM, infrav1.VMProvisionedCondition))
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

const (
	invalidUUID       = "fakeuuid-1472-547c-b873-6dc7883fb6cb"
	validBiosUUID     = "265104de-1472-547c-b873-6dc7883fb6cb"
	validInstanceUUID = "b4689bed-97f0-5bcd-8a4c-07477cc8f06f"
	invalidFolder     = "/DC0/vmNot/"
	invalidVMName     = "DC0_H0_VMNot"
	validFolder       = "/DC0/vm/"
	validVMName       = "DC0_H0_VM0"
)

func TestFindVMWithBiosUUID(t *testing.T) {
	g := NewWithT(t)

	sim, authSession, err := configureSimulatorAndSession(simulator.VPX(), []string{})
	g.Expect(err).NotTo(HaveOccurred(), "failed to get configure simulator or session")
	t.Cleanup(func() {
		sim.Destroy()
	})

	testCases := []struct {
		name       string
		uuid       string
		session    *session.Session
		errMatcher func(g *GomegaWithT, err error)
		refMatcher func(g *GomegaWithT, ref *types.ManagedObjectReference)
	}{
		{
			name:    "when context has a bios uuid and its valid",
			uuid:    validBiosUUID,
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).ToNot(BeNil())
			},
		},
		{
			name:    "when context has a bios uuid but its not valid",
			uuid:    invalidUUID,
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(isNotFound(err)).To(BeTrue())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).To(Equal(&types.ManagedObjectReference{}))
			},
		},
		{
			name:    "when context does not have a bios uuid",
			uuid:    "",
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(isNotApplicable(err)).To(BeTrue())
				g.Expect(err.Error()).To(ContainSubstring("bios uuid"))
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).To(Equal(&types.ManagedObjectReference{}))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			vmCtx := &context.VMContext{
				ControllerContext: fake.NewControllerContext(fake.NewControllerManagerContext()),
				Logger:            logr.Discard(),
				VSphereVM: &infrav1.VSphereVM{
					Spec: infrav1.VSphereVMSpec{
						BiosUUID: testCase.uuid,
					},
				},
				Session: testCase.session,
			}
			vm, err := findVMWithBiosUUID(vmCtx)
			testCase.errMatcher(g, err)
			testCase.refMatcher(g, &vm)
		})
	}
}

func TestFindVMWithInstanceUUID(t *testing.T) {
	g := NewWithT(t)

	sim, authSession, err := configureSimulatorAndSession(simulator.VPX(), []string{})
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() {
		sim.Destroy()
	})

	testCases := []struct {
		name       string
		iUUID      string
		session    *session.Session
		errMatcher func(g *GomegaWithT, err error)
		refMatcher func(g *GomegaWithT, ref *types.ManagedObjectReference)
	}{
		{
			name:    "when context has a instance uuid and its valid",
			iUUID:   validInstanceUUID,
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).ToNot(Equal(&types.ManagedObjectReference{}))
			},
		},
		{
			name:    "when context has an instance uuid but its not valid",
			iUUID:   invalidUUID,
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(isNotFound(err)).To(BeTrue())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).To(Equal(&types.ManagedObjectReference{}))
			},
		},
		{
			name:    "when context does not have a instance uuid",
			iUUID:   "",
			session: authSession,
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).To(Equal(&types.ManagedObjectReference{}))
			},
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(isNotApplicable(err)).To(BeTrue())
				g.Expect(err.Error()).To(ContainSubstring("instance uuid"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			vmCtx := &context.VMContext{
				ControllerContext: fake.NewControllerContext(fake.NewControllerManagerContext()),
				Logger:            logr.Discard(),
				VSphereVM: &infrav1.VSphereVM{
					ObjectMeta: metav1.ObjectMeta{
						UID: types2.UID(testCase.iUUID),
					},
				},
				Session: testCase.session,
			}
			vm, err := findVMWithInstanceUUID(vmCtx)
			testCase.errMatcher(g, err)
			testCase.refMatcher(g, &vm)
		})
	}
}

func TestFindVMWithInventoryPath(t *testing.T) {
	g := NewWithT(t)

	sim, authSession, err := configureSimulatorAndSession(simulator.VPX(), []string{})
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() {
		sim.Destroy()
	})

	testCases := []struct {
		name       string
		folder     string
		vmName     string
		session    *session.Session
		errMatcher func(g *GomegaWithT, err error)
		refMatcher func(g *GomegaWithT, ref *types.ManagedObjectReference)
	}{
		{
			name:    "when the context has valid folder and VM names pointing to a VM",
			folder:  validFolder,
			vmName:  validVMName,
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).ToNot(Equal(&types.ManagedObjectReference{}))
			},
		},
		{
			name:    "when the context has an invalid VM folder, valid VM name",
			folder:  invalidFolder,
			vmName:  validVMName,
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("folder '/DC0/vmNot/' not found"))
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).To(Equal(&types.ManagedObjectReference{}))
			},
		},
		{
			name:    "when the context has a valid folder but an invalid VM name",
			folder:  validFolder,
			vmName:  invalidVMName,
			session: authSession,
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(isNotFound(err)).To(BeTrue())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).To(Equal(&types.ManagedObjectReference{}))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			vmCtx := &context.VMContext{
				ControllerContext: fake.NewControllerContext(fake.NewControllerManagerContext()),
				Logger:            logr.Discard(),
				VSphereVM: &infrav1.VSphereVM{
					ObjectMeta: metav1.ObjectMeta{
						Name: testCase.vmName,
					},
					Spec: infrav1.VSphereVMSpec{
						VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{
							Folder: testCase.folder,
						},
					},
				},
				Session: testCase.session,
			}
			vm, err := findVMWithInventoryPath(vmCtx)
			testCase.errMatcher(g, err)
			testCase.refMatcher(g, &vm)
		})
	}
}

func TestFindVM(t *testing.T) {
	g := NewWithT(t)

	sim, authSession, err := configureSimulatorAndSession(simulator.VPX(), []string{})
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() {
		sim.Destroy()
	})

	baseVMCtx := func() context.VMContext {
		return context.VMContext{ControllerContext: fake.NewControllerContext(fake.NewControllerManagerContext()),
			Logger: logr.Discard(),
			VSphereVM: &infrav1.VSphereVM{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: infrav1.VSphereVMSpec{
					VirtualMachineCloneSpec: infrav1.VirtualMachineCloneSpec{},
				},
			},
			Session: authSession,
		}
	}

	testCases := []struct {
		name       string
		vmCtx      *context.VMContext
		errMatcher func(g *GomegaWithT, err error)
		refMatcher func(g *GomegaWithT, ref *types.ManagedObjectReference)
	}{
		{
			name: "when the context has no valid bios uuid, instance uuid and vm path",
			vmCtx: func() *context.VMContext {
				vmCtx := baseVMCtx()
				return &vmCtx
			}(),
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("vm with inventory path /DC0/vm not found"), "error should indicate that the last attempt was finding the vm by path")
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).To(Equal(&types.ManagedObjectReference{}))
			},
		},
		{
			name: "when the context has a valid bios uuid but no instance uuid and vm path",
			vmCtx: func() *context.VMContext {
				vmCtx := baseVMCtx()
				vmCtx.VSphereVM.Spec.BiosUUID = validBiosUUID
				return &vmCtx
			}(),
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).ToNot(Equal(&types.ManagedObjectReference{}), "should have found the vm")
			},
		},
		{
			name: "when the context has a valid instance uuid but no bios uuid and vm path",
			vmCtx: func() *context.VMContext {
				vmCtx := baseVMCtx()
				vmCtx.VSphereVM.UID = validInstanceUUID
				return &vmCtx
			}(),
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).ToNot(Equal(&types.ManagedObjectReference{}), "should have found the vm by instance uuid")
			},
		},
		{
			name: "when the context has a valid vm path but no bios or instance uuid",
			vmCtx: func() *context.VMContext {
				vmCtx := baseVMCtx()
				vmCtx.VSphereVM.Spec.Folder = validFolder
				vmCtx.VSphereVM.ObjectMeta.Name = validVMName
				return &vmCtx
			}(),
			errMatcher: func(g *GomegaWithT, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
			refMatcher: func(g *GomegaWithT, ref *types.ManagedObjectReference) {
				g.Expect(ref).ToNot(Equal(&types.ManagedObjectReference{}), "should have found the vm by path")
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			vm, err := findVM(testCase.vmCtx)
			testCase.errMatcher(g, err)
			testCase.refMatcher(g, &vm)
		})
	}
}

// configureSimulatorAndSession creates and configures a VC simulator and an associated session.
func configureSimulatorAndSession(model *simulator.Model, setupCommands []string) (*vcsim.Simulator, *session.Session, error) {
	sim, err := vcsim.NewBuilder().WithModel(model).WithOperations(setupCommands...).Build()
	if err != nil {
		return nil, nil, err
	}

	authSession, err := session.GetOrCreate(
		context2.Background(),
		session.NewParams().
			WithServer(sim.ServerURL().Host).
			WithUserInfo(sim.Username(), sim.Password()).
			WithDatacenter("*"))

	return sim, authSession, err
}
