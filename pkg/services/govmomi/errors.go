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

package govmomi

import (
	"fmt"

	"github.com/vmware/govmomi/find"
)

// notFoundError is returned by the findVM function when a VM is not found.
type notFoundError struct {
	uuid            string
	byInventoryPath string
}

func (e notFoundError) Error() string {
	if e.byInventoryPath != "" {
		return fmt.Sprintf("vm with inventory path %s not found", e.byInventoryPath)
	}
	return fmt.Sprintf("vm with bios uuid %s not found", e.uuid)
}

func isNotFound(err error) bool {
	switch err.(type) {
	case notFoundError, *notFoundError:
		return true
	default:
		return false
	}
}

func isFolderNotFound(err error) bool {
	switch err.(type) {
	case *find.NotFoundError:
		return true
	default:
		return false
	}
}

func isVirtualMachineNotFound(err error) bool {
	switch err.(type) {
	case *find.NotFoundError:
		return true
	default:
		return false
	}
}

func wasNotFoundByBIOSUUID(err error) bool {
	switch err.(type) {
	case notFoundError, *notFoundError:
		if err.(notFoundError).uuid != "" && err.(notFoundError).byInventoryPath == "" {
			return true
		}
		return false
	default:
		return false
	}
}
