/*
Copyright 2017 The Kubernetes Authors.

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
	"fmt"
	"sync"
)

// This implementation is based on https://github.com/kubernetes-sigs/boskos/blob/59dbd6c27f19fbd469b62b22177f22dc0a5d52dd/storage/storage.go
// We didn't want to import it directly to avoid dependencies on old controller-runtime / client-go versions.

// PersistenceLayer defines a simple interface to persists Boskos Information.
type PersistenceLayer interface {
	Add(r Resource) error
	Delete(name string) error
	Update(r Resource) (Resource, error)
	Get(name string) (Resource, error)
	List() ([]Resource, error)
}

type inMemoryStore struct {
	resources map[string]Resource
	lock      sync.RWMutex
}

// NewMemoryStorage creates an in memory persistence layer.
func NewMemoryStorage() PersistenceLayer {
	return &inMemoryStore{
		resources: map[string]Resource{},
	}
}

func (im *inMemoryStore) Add(r Resource) error {
	im.lock.Lock()
	defer im.lock.Unlock()
	_, ok := im.resources[r.Name]
	if ok {
		return fmt.Errorf("resource %s already exists", r.Name)
	}
	im.resources[r.Name] = r
	return nil
}

func (im *inMemoryStore) Delete(name string) error {
	im.lock.Lock()
	defer im.lock.Unlock()
	_, ok := im.resources[name]
	if !ok {
		return fmt.Errorf("cannot find item %s", name)
	}
	delete(im.resources, name)
	return nil
}

func (im *inMemoryStore) Update(r Resource) (Resource, error) {
	im.lock.Lock()
	defer im.lock.Unlock()
	_, ok := im.resources[r.Name]
	if !ok {
		return Resource{}, fmt.Errorf("cannot find item %s", r.Name)
	}
	im.resources[r.Name] = r
	return r, nil
}

func (im *inMemoryStore) Get(name string) (Resource, error) {
	im.lock.RLock()
	defer im.lock.RUnlock()
	r, ok := im.resources[name]
	if !ok {
		return Resource{}, fmt.Errorf("cannot find item %s", name)
	}
	return r, nil
}

func (im *inMemoryStore) List() ([]Resource, error) {
	im.lock.RLock()
	defer im.lock.RUnlock()
	resources := []Resource{}
	for _, r := range im.resources {
		resources = append(resources, r)
	}
	return resources, nil
}
