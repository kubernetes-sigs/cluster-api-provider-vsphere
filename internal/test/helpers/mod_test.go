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

package helpers

import (
	"os"
	"testing"
)

func TestMod_FindDependencyVersion(t *testing.T) {
	goModData := `module sigs.k8s.io/dummy-project

go 1.17

require (
    github.com/foo/bar v1.0.0
	github.com/foo/baz v0.9.1
)

replace (
	github.com/foo/bar v1.0.0 => github.com/foo/bar v1.0.1
)
`
	tempPath, err := createTempGoMod(goModData)
	if err != nil {
		t.Fatal("failed to create test file")
	}
	defer os.RemoveAll(tempPath)

	m, err := NewMod(tempPath)
	if err != nil {
		t.Fatalf("failed to init %s", err)
	}

	t.Run("find version for existing package with replace", func(t *testing.T) {
		name := "github.com/foo/bar"
		ver, err := m.FindDependencyVersion(name)
		if err != nil {
			t.Fatalf("failed to find version for package %s", name)
		}
		if ver != "v1.0.1" {
			t.Fatalf("incorrect version %s", ver)
		}
	})

	t.Run("find version for non-existing package", func(t *testing.T) {
		name := "sigs.k8s.io/no-such-project"
		ver, err := m.FindDependencyVersion(name)
		if err == nil || ver != "" {
			t.Fatalf("found not existent package %s", name)
		}
	})

	t.Run("find version for existing package without replace", func(t *testing.T) {
		name := "github.com/foo/baz"
		ver, err := m.FindDependencyVersion(name)
		if err != nil {
			t.Fatalf("failed to find version for package %s", name)
		}
		if ver != "v0.9.1" {
			t.Fatalf("incorrect version %s", ver)
		}
	})
}

func createTempGoMod(data string) (string, error) {
	dir, err := os.MkdirTemp("", "parse-mod-")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "test.mod")
	if err != nil {
		return "", err
	}

	if _, err := f.WriteString(data); err != nil {
		return "", err
	}
	defer f.Close()

	return f.Name(), nil
}
