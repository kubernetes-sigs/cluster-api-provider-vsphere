/*
Copyright 2024 The Kubernetes Authors.

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

package boskos

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/yaml"
)

// This implementation is based on https://github.com/kubernetes-sigs/boskos/blob/59dbd6c27f19fbd469b62b22177f22dc0a5d52dd/common/common.go#L34
// We didn't want to import it directly to avoid dependencies on old controller-runtime / client-go versions.

const (
	// Busy state defines a resource being used.
	Busy = "busy"
	// Cleaning state defines a resource being cleaned.
	Cleaning = "cleaning"
	// Dirty state defines a resource that needs cleaning.
	Dirty = "dirty"
	// Free state defines a resource that is usable.
	Free = "free"
	// Leased state defines a resource being leased in order to make a new resource.
	Leased = "leased"
	// ToBeDeleted is used for resources about to be deleted, they will be verified by a cleaner which mark them as tombstone.
	ToBeDeleted = "toBeDeleted"
	// Tombstone is the state in which a resource can safely be deleted.
	Tombstone = "tombstone"
	// Other is used to agglomerate unspecified states for metrics reporting.
	Other = "other"
)

// UserData is a map of Name to user defined interface, serialized into a string.
type UserData struct {
	sync.Map
}

// UserDataMap is the standard Map version of UserMap, it is used to ease UserMap creation.
type UserDataMap map[string]string

// Resource abstracts any resource type that can be tracked by boskos.
type Resource struct {
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	State      string    `json:"state"`
	Owner      string    `json:"owner"`
	LastUpdate time.Time `json:"lastupdate"`
	// Customized UserData
	UserData *UserData `json:"userdata"`
	// Used to clean up dynamic resources
	ExpirationDate *time.Time `json:"expiration-date,omitempty"`
}

// Metric contains analytics about a specific resource type.
type Metric struct {
	Type    string         `json:"type"`
	Current map[string]int `json:"current"`
	Owners  map[string]int `json:"owner"`
	// TODO: implements state transition metrics
}

// UserDataNotFound will be returned if requested resource does not exist.
type UserDataNotFound struct {
	ID string
}

func (ud *UserDataNotFound) Error() string {
	return fmt.Sprintf("user data ID %s does not exist", ud.ID)
}

// UnmarshalJSON implements JSON Unmarshaler interface.
func (ud *UserData) UnmarshalJSON(data []byte) error {
	tmpMap := UserDataMap{}
	if err := json.Unmarshal(data, &tmpMap); err != nil {
		return err
	}
	ud.FromMap(tmpMap)
	return nil
}

// MarshalJSON implements JSON Marshaler interface.
func (ud *UserData) MarshalJSON() ([]byte, error) {
	return json.Marshal(ud.ToMap())
}

// Extract unmarshalls a string a given struct if it exists.
func (ud *UserData) Extract(id string, out interface{}) error {
	content, ok := ud.Load(id)
	if !ok {
		return &UserDataNotFound{id}
	}
	return yaml.Unmarshal([]byte(content.(string)), out)
}

// Update updates existing UserData with new UserData.
// If a key as an empty string, the key will be deleted.
func (ud *UserData) Update(newUserData *UserData) *UserData {
	if newUserData == nil {
		return ud
	}
	newUserData.Range(func(key, value interface{}) bool {
		if value.(string) != "" {
			ud.Store(key, value)
		} else {
			ud.Delete(key)
		}
		return true
	})
	return ud
}

// ToMap converts a UserData to UserDataMap.
func (ud *UserData) ToMap() UserDataMap {
	if ud == nil {
		return nil
	}
	m := UserDataMap{}
	ud.Range(func(key, value interface{}) bool {
		m[key.(string)] = value.(string)
		return true
	})
	return m
}

// FromMap feels updates user data from a map.
func (ud *UserData) FromMap(m UserDataMap) {
	for key, value := range m {
		ud.Store(key, value)
	}
}

// ResourceTypeNotFoundMessage returns a resource type not found message.
func ResourceTypeNotFoundMessage(rType string) string {
	return fmt.Sprintf("resource type %q does not exist", rType)
}
