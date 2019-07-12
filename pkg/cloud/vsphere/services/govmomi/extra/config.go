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

package extra

import (
	"encoding/base64"

	"github.com/vmware/govmomi/vim25/types"
)

// Config is data used with a VM's guestInfo RPC interface.
type Config []types.BaseOptionValue

// SetCloudInitUserData sets the cloud init user data at the key
// "guestinfo.userdata" as a base64-encoded string.
func (e *Config) SetCloudInitUserData(data []byte) {
	*e = append(*e,
		&types.OptionValue{
			Key:   "guestinfo.userdata",
			Value: base64.StdEncoding.EncodeToString(data),
		},
		&types.OptionValue{
			Key:   "guestinfo.userdata.encoding",
			Value: "base64",
		},
	)
}

// SetCloudInitMetadata sets the cloud init user data at the key
// "guestinfo.metadata" as a base64-encoded string.
func (e *Config) SetCloudInitMetadata(data []byte) {
	*e = append(*e,
		&types.OptionValue{
			Key:   "guestinfo.metadata",
			Value: base64.StdEncoding.EncodeToString(data),
		},
		&types.OptionValue{
			Key:   "guestinfo.metadata.encoding",
			Value: "base64",
		},
	)
}
