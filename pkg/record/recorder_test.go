/*
Copyright 2019 The Kubernetes Authors.

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

package record_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apirecord "k8s.io/client-go/tools/record"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
)

var _ = Describe("Event utils", func() {
	fakeRecorder := apirecord.NewFakeRecorder(100)
	recorder := record.New(fakeRecorder)

	Context("Publish event", func() {
		It("should not publish an event", func() {
			var err error
			recorder.EmitEvent(nil, "Create", err, true)
			Expect(len(fakeRecorder.Events)).Should(Equal(0))
		})

		It("should publish a success event", func() {
			var err error
			recorder.EmitEvent(nil, "Create", err, false)
			Expect(len(fakeRecorder.Events)).Should(Equal(1))
			event := <-fakeRecorder.Events
			Expect(event).Should(Equal("Normal CreateSuccess Create success"))
		})

		It("should publish a failure event", func() {
			err := errors.New("something wrong")
			recorder.EmitEvent(nil, "Create", err, false)
			Expect(len(fakeRecorder.Events)).Should(Equal(1))
			event := <-fakeRecorder.Events
			Expect(event).Should(Equal("Warning CreateFailure something wrong"))
		})

		It("should use Sprintf to format event message", func() {
			message := "a % message: should call"
			fmtArgs := []interface{}{"formatted", "Sprintf"}

			recorder.Eventf(nil, "Create", message, fmtArgs...)
			recorder.Warnf(nil, "Create", message, fmtArgs...)
			Expect(len(fakeRecorder.Events)).To(Equal(2))
			eventFmt := <-fakeRecorder.Events
			warnFmt := <-fakeRecorder.Events

			recorder.Event(nil, "Create", message)
			recorder.Warn(nil, "Create", message)
			Expect(len(fakeRecorder.Events)).To(Equal(2))
			eventNoFmt := <-fakeRecorder.Events
			warnNoFmt := <-fakeRecorder.Events

			Expect(eventFmt).To(Equal(fmt.Sprintf(eventNoFmt, fmtArgs...)), "Eventf should call Sprintf to format the message under-the-hood")
			Expect(warnFmt).To(Equal(fmt.Sprintf(warnNoFmt, fmtArgs...)), "Warnf should call Sprintf to format the message under-the-hood")
		})
	})
})
