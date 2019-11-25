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
	})
})
