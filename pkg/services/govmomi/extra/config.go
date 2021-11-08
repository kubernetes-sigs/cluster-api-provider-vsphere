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

// SetCustomVMXKeys sets the custom VMX keys as
// OptionValues in extraConfig.
func (e *Config) SetCustomVMXKeys(customKeys map[string]string) error {
	for k, v := range customKeys {
		*e = append(*e, &types.OptionValue{
			Key:   k,
			Value: v,
		})
	}
	return nil
}

// SetCloudInitUserData sets the cloud init user data at the key
// "guestinfo.userdata" as a base64-encoded string.
func (e *Config) SetCloudInitUserData(data []byte) error {
	*e = append(*e,
		&types.OptionValue{
			Key:   "guestinfo.userdata",
			Value: e.encode(data),
		},
		&types.OptionValue{
			Key:   "guestinfo.userdata.encoding",
			Value: "base64",
		},
	)
	return nil
}

// SetCloudInitMetadata sets the cloud init user data at the key
// "guestinfo.metadata" as a base64-encoded string.
func (e *Config) SetCloudInitMetadata(data []byte) error {
	*e = append(*e,
		&types.OptionValue{
			Key:   "guestinfo.metadata",
			Value: e.encode(data),
		},
		&types.OptionValue{
			Key:   "guestinfo.metadata.encoding",
			Value: "base64",
		},
	)

	return nil
}

// encode first attempts to decode the data as many times as necessary
// to ensure it is plain-text before returning the result as a base64
// encoded string.
func (e *Config) encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	for {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			break
		}
		data = decoded
	}
	return base64.StdEncoding.EncodeToString(data)
}
