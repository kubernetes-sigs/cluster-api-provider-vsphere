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

package haproxy

import (
	"net/http"

	hapi "sigs.k8s.io/cluster-api-provider-vsphere/contrib/haproxy/openapi"
)

// AddrOfInt32 returns the address of the provided int32 value.
func AddrOfInt32(i int32) *int32 {
	return &i
}

// IsNotFound returns true if the provided error indicates a resource is
// not found.
func IsNotFound(err error) bool {
	return isHTTPStatus(err, http.StatusNotFound)
}

// IsConflict returns true if the provided error indicates a resource is
// in conflict with an existing resource.
func IsConflict(err error) bool {
	return isHTTPStatus(err, http.StatusConflict)
}

func isHTTPStatus(err error, status int32) bool {
	if err == nil {
		return false
	}
	openapiErr, ok := err.(hapi.GenericOpenAPIError)
	if !ok {
		return false
	}
	modelErr, ok := openapiErr.Model().(hapi.ModelError)
	if !ok {
		return false
	}
	return modelErr.Code != nil && *modelErr.Code == status
}
