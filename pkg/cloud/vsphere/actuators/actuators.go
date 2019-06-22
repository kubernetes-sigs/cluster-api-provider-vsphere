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

package actuators

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	clustererr "sigs.k8s.io/cluster-api/pkg/controller/error"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/record"
)

type patchContext interface {
	fmt.Stringer
	GetObject() runtime.Object
	GetLogger() logr.Logger
	Patch() error
}

// PatchAndHandleError is used by actuators to patch objects and handle any
// errors that may occur.
func PatchAndHandleError(ctx patchContext, opName string, opErr error) error {

	err := opErr

	// Attempt to patch the object. If it fails then requeue the operation.
	if patchErr := ctx.Patch(); patchErr != nil {
		err = errors.Wrapf(
			&clustererr.RequeueAfterError{RequeueAfter: constants.DefaultRequeue},
			"opErr=%q patchErr=%q", opErr, patchErr)
	}

	// Always make sure an underlying requeue error is returned if one is present
	// and log the op error if one is replaced with the requeue error.
	requeueErr, isRequeueErr := errors.Cause(err).(*clustererr.RequeueAfterError)
	if isRequeueErr {
		// If the requeue error is not the outer-most error then log the outer-most
		// error since it will be dropped in favor of the requeue.
		if requeueErr != err {
			ctx.GetLogger().V(6).Info("requeue after error", "object", ctx.String(), "error", err)
		}
		err = requeueErr
	}

	if err == nil {
		record.Event(ctx.GetObject(), opName+"Success", opName+" success")
	} else if isRequeueErr {
		record.Event(ctx.GetObject(), opName+"Requeue", "requeued "+opName)
	} else {
		record.Warn(ctx.GetObject(), opName+"Failure", err.Error())
	}

	return err
}
