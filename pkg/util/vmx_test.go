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
	"testing"

	"github.com/onsi/gomega"
)

func Test_LessThan(t *testing.T) {
	t.Run("when version1 < version2", func(t *testing.T) {
		isLessThan, err := LessThan("vmx-15", "vmx-17")
		if err != nil {
			t.Errorf("no error expected: %s", err)
		}
		if !isLessThan {
			t.Errorf("expected: true, actual: %t", isLessThan)
		}
	})
	t.Run("when version1 > version2", func(t *testing.T) {
		isLessThan, err := LessThan("vmx-19", "vmx-17")
		if err != nil {
			t.Errorf("no error expected: %s", err)
		}
		if isLessThan {
			t.Errorf("expected: false, actual: %t", isLessThan)
		}
	})
	t.Run("when input version1 is invalid", func(t *testing.T) {
		_, err := LessThan("vmx-abc", "vmx-17")
		if err == nil {
			t.Error("error expected due to incorrect inputs")
		}
	})
	t.Run("when input version2 is invalid", func(t *testing.T) {
		_, err := LessThan("vmx-17", "vmx-abc")
		if err == nil {
			t.Error("error expected due to incorrect inputs")
		}
	})
}

func Test_ParseHardwareVersion(t *testing.T) {
	t.Run("for valid input", func(t *testing.T) {
		g := gomega.NewWithT(t)

		version, err := ParseHardwareVersion("vmx-17")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(version).To(gomega.Equal(17))
	})

	t.Run("for invalid input", func(t *testing.T) {
		g := gomega.NewWithT(t)

		_, err := ParseHardwareVersion("vmx-abc")
		g.Expect(err).To(gomega.HaveOccurred())
	})

	t.Run("for input without prefix", func(t *testing.T) {
		g := gomega.NewWithT(t)

		version, err := ParseHardwareVersion("18")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(version).To(gomega.Equal(18))
	})
}
