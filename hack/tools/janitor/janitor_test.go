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

package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/simulator/vpx"
	"github.com/vmware/govmomi/view"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
)

func setup(ctx context.Context, t *testing.T) (*vSphereClients, *vcsim.Simulator) {
	t.Helper()
	model := &simulator.Model{
		ServiceContent: vpx.ServiceContent,
		RootFolder:     vpx.RootFolder,
		Autostart:      true,
		Datacenter:     1,
		Portgroup:      1,
		Host:           1,
		Cluster:        1,
		ClusterHost:    3,
		DelayConfig:    simulator.DelayConfig{},
	}

	vcsim, err := vcsim.NewBuilder().WithModel(model).Build()
	if err != nil {
		panic(fmt.Sprintf("unable to create simulator %s", err))
	}

	fmt.Printf(" export GOVC_URL=%s\n", vcsim.ServerURL())
	fmt.Printf(" export GOVC_USERNAME=%s\n", vcsim.Username())
	fmt.Printf(" export GOVC_PASSWORD=%s\n", vcsim.Password())
	fmt.Printf(" export GOVC_INSECURE=true\n")

	clients, err := newVSphereClients(ctx, getVSphereClientInput{
		Username:  vcsim.Username(),
		Password:  vcsim.Password(),
		Server:    vcsim.ServerURL().String(),
		UserAgent: "capv-janitor-test",
	})
	if err != nil {
		panic(err)
	}

	t.Cleanup(vcsim.Destroy)

	return clients, vcsim
}

func setupTestCase(g *gomega.WithT, sim *vcsim.Simulator, objects []*vcsimObject) (string, map[string]bool) {
	g.THelper()

	relativePath := rand.String(10)

	baseRP := vcsimResourcePool("")
	baseFolder := vcsimFolder("")
	baseDatastore := vcsimDatastore("", os.TempDir())
	// Create base objects for the test case
	g.Expect(baseRP.Create(sim, relativePath)).To(gomega.Succeed())
	g.Expect(baseFolder.Create(sim, relativePath)).To(gomega.Succeed())
	g.Expect(baseDatastore.Create(sim, relativePath)).To(gomega.Succeed())

	createdObjects := map[string]bool{}

	// Create objects
	for _, object := range objects {
		createdObjects[path.Join(object.objectType, object.pathSuffix)] = true
		g.Expect(object.Create(sim, relativePath)).To(gomega.Succeed())
	}

	return relativePath, createdObjects
}

const (
	folderBase       = "/DC0/vm"
	resourcePoolBase = "/DC0/host/DC0_C0/Resources"
)

func Test_janitor_deleteVSphereVMs(t *testing.T) {
	ctx := context.Background()
	ctx = ctrl.LoggerInto(ctx, klog.Background())

	// Initialize and start vcsim
	clients, sim := setup(ctx, t)

	deleteAll := time.Now().Add(time.Hour * 1)
	deleteNone := time.Now()

	tests := []struct {
		name            string
		objects         []*vcsimObject
		maxCreationDate time.Time
		wantErr         bool
		want            map[string]bool
	}{
		{
			name: "delete all VMs",
			objects: []*vcsimObject{
				vcsimVirtualMachine("foo"),
			},
			maxCreationDate: deleteAll,
			wantErr:         false,
			want:            nil,
		},
		{
			name: "delete no VMs",
			objects: []*vcsimObject{
				vcsimVirtualMachine("foo"),
			},
			maxCreationDate: deleteNone,
			wantErr:         false,
			want: map[string]bool{
				"VirtualMachine/foo": true,
			},
		},
		{
			name: "recursive vm deletion",
			objects: []*vcsimObject{
				vcsimResourcePool("a"),
				vcsimFolder("a"),
				vcsimResourcePool("a/b"),
				vcsimFolder("a/b"),
				vcsimResourcePool("a/b/c"),
				vcsimFolder("a/b/c"),
				vcsimVirtualMachine("foo"),
				vcsimVirtualMachine("a/bar"),
				vcsimVirtualMachine("a/b/c/foobar"),
			},
			maxCreationDate: deleteAll,
			wantErr:         false,
			want: map[string]bool{
				"ResourcePool/a":     true,
				"ResourcePool/a/b":   true,
				"ResourcePool/a/b/c": true,
				"Folder/a":           true,
				"Folder/a/b":         true,
				"Folder/a/b/c":       true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			relativePath, _ := setupTestCase(g, sim, tt.objects)

			s := &janitor{
				dryRun:          false,
				maxCreationDate: tt.maxCreationDate,
				vSphereClients:  clients,
			}

			// use folder created for this test case as inventoryPath
			inventoryPath := vcsimFolder("").Path(relativePath)

			err := s.deleteVSphereVMs(ctx, inventoryPath)
			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			}

			// Ensure the expected objects still exists
			existingObjects, err := recursiveListFoldersAndResourcePools(ctx, relativePath, clients.Govmomi, clients.Finder, clients.ViewManager)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			if tt.want != nil {
				g.Expect(existingObjects).To(gomega.BeEquivalentTo(tt.want))
			} else {
				g.Expect(existingObjects).To(gomega.BeEmpty())
			}
		})
	}
}

func Test_janitor_deleteObjectChildren(t *testing.T) {
	ctx := context.Background()
	ctx = ctrl.LoggerInto(ctx, klog.Background())

	// Initialize and start vcsim
	clients, sim := setup(ctx, t)

	tests := []struct {
		name       string
		basePath   string
		objectType string
		objects    []*vcsimObject
		wantErr    bool
		want       map[string]bool
	}{
		{
			name:       "should preserve resource pool if it contains a vm and delete empty resource pools",
			basePath:   resourcePoolBase,
			objectType: "ResourcePool",
			objects: []*vcsimObject{
				vcsimResourcePool("a"),
				vcsimResourcePool("b"),
				vcsimFolder("a"),
				vcsimVirtualMachine("a/foo"),
			},
			want: map[string]bool{
				"Folder/a":             true,
				"ResourcePool/a":       true,
				"VirtualMachine/a/foo": true,
			},
		},
		{
			name:       "should preserve folder if it contains a vm and delete empty folders",
			basePath:   folderBase,
			objectType: "Folder",
			objects: []*vcsimObject{
				vcsimResourcePool("a"),
				vcsimFolder("a"),
				vcsimFolder("b"),
				vcsimVirtualMachine("a/foo"),
			},
			want: map[string]bool{
				"Folder/a":             true,
				"ResourcePool/a":       true,
				"VirtualMachine/a/foo": true,
			},
		},
		{
			name:       "no-op",
			basePath:   resourcePoolBase,
			objectType: "ResourcePool",
			objects:    []*vcsimObject{},
		},
		{
			name:       "single resource pool",
			basePath:   resourcePoolBase,
			objectType: "ResourcePool",
			objects: []*vcsimObject{
				vcsimResourcePool("a"),
			},
		},
		{
			name:       "multiple nested resource pools",
			basePath:   resourcePoolBase,
			objectType: "ResourcePool",
			objects: []*vcsimObject{
				vcsimResourcePool("a"),
				vcsimResourcePool("a/b"),
				vcsimResourcePool("a/b/c"),
				vcsimResourcePool("d"),
				vcsimResourcePool("d/e"),
				vcsimResourcePool("f"),
			},
		},
		{
			name:       "no-op",
			basePath:   folderBase,
			objectType: "Folder",
			objects:    []*vcsimObject{},
		},
		{
			name:       "single folder",
			basePath:   folderBase,
			objectType: "Folder",
			objects: []*vcsimObject{
				vcsimFolder("a"),
			},
		},
		{
			name:       "multiple nested folders",
			basePath:   folderBase,
			objectType: "Folder",
			objects: []*vcsimObject{
				vcsimFolder("a"),
				vcsimFolder("a/b"),
				vcsimFolder("a/b/c"),
				vcsimFolder("d"),
				vcsimFolder("d/e"),
				vcsimFolder("f"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			relativePath, wantMarkedObjects := setupTestCase(g, sim, tt.objects)

			inventoryPath := path.Join(tt.basePath, relativePath)

			s := &janitor{
				dryRun:          false,
				maxCreationDate: time.Now().Add(time.Hour * 1),
				vSphereClients:  clients,
			}

			// Run first iteration which should only tag the resource pools with a timestamp.
			g.Expect(s.deleteObjectChildren(ctx, inventoryPath, tt.objectType)).To(gomega.Succeed())
			existingObjects, err := recursiveListFoldersAndResourcePools(ctx, relativePath, clients.Govmomi, clients.Finder, clients.ViewManager)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(existingObjects).To(gomega.BeEquivalentTo(wantMarkedObjects))

			// Run second iteration which should destroy the resource pools with a timestamp.
			g.Expect(s.deleteObjectChildren(ctx, inventoryPath, tt.objectType)).To(gomega.Succeed())
			existingObjects, err = recursiveListFoldersAndResourcePools(ctx, relativePath, clients.Govmomi, clients.Finder, clients.ViewManager)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			if tt.want != nil {
				g.Expect(existingObjects).To(gomega.BeEquivalentTo(tt.want))
			} else {
				g.Expect(existingObjects).To(gomega.BeEmpty())
			}

			// Ensure the parent object still exists
			assertObjectExists(ctx, g, clients.Finder, inventoryPath)
		})
	}
}

func Test_janitor_CleanupVSphere(t *testing.T) {
	ctx := context.Background()
	ctx = ctrl.LoggerInto(ctx, klog.Background())

	// Initialize and start vcsim
	clients, sim := setup(ctx, t)

	deleteAll := time.Now().Add(time.Hour * 1)

	tests := []struct {
		name               string
		dryRun             bool
		maxCreationDate    time.Time
		objects            []*vcsimObject
		wantAfterFirstRun  map[string]bool
		wantAfterSecondRun map[string]bool
	}{
		{
			name:               "no-op",
			dryRun:             false,
			maxCreationDate:    deleteAll,
			objects:            nil,
			wantAfterFirstRun:  map[string]bool{},
			wantAfterSecondRun: map[string]bool{},
		},
		{
			name:               "dryRun: no-op",
			dryRun:             true,
			maxCreationDate:    deleteAll,
			objects:            nil,
			wantAfterFirstRun:  map[string]bool{},
			wantAfterSecondRun: map[string]bool{},
		},
		{
			name:            "delete everything",
			dryRun:          false,
			maxCreationDate: deleteAll,
			objects: []*vcsimObject{
				vcsimFolder("a"),
				vcsimResourcePool("a"),
				vcsimVirtualMachine("a/b"),
				vcsimFolder("c"),
				vcsimResourcePool("c"),
			},
			wantAfterFirstRun: map[string]bool{
				"Folder/a":       true,
				"Folder/c":       true,
				"ResourcePool/a": true,
				"ResourcePool/c": true,
			},
			wantAfterSecondRun: map[string]bool{},
		},
		{
			name:            "dryRun: would delete everything",
			dryRun:          true,
			maxCreationDate: deleteAll,
			objects: []*vcsimObject{
				vcsimFolder("a"),
				vcsimResourcePool("a"),
				vcsimVirtualMachine("a/b"),
				vcsimFolder("c"),
				vcsimResourcePool("c"),
			},
			wantAfterFirstRun: map[string]bool{
				"Folder/a":           true,
				"Folder/c":           true,
				"ResourcePool/a":     true,
				"ResourcePool/c":     true,
				"VirtualMachine/a/b": true,
			},
			wantAfterSecondRun: map[string]bool{
				"Folder/a":           true,
				"Folder/c":           true,
				"ResourcePool/a":     true,
				"ResourcePool/c":     true,
				"VirtualMachine/a/b": true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			relativePath, _ := setupTestCase(g, sim, tt.objects)

			s := &janitor{
				dryRun:          tt.dryRun,
				maxCreationDate: tt.maxCreationDate,
				vSphereClients:  clients,
			}

			folder := vcsimFolder("").Path(relativePath)
			resourcePool := vcsimResourcePool("").Path(relativePath)

			folders := []string{folder}
			resourcePools := []string{resourcePool}

			g.Expect(s.cleanupVSphere(ctx, folders, resourcePools, folders)).To(gomega.Succeed())
			existingObjects, err := recursiveListFoldersAndResourcePools(ctx, relativePath, clients.Govmomi, clients.Finder, clients.ViewManager)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(existingObjects).To(gomega.BeEquivalentTo(tt.wantAfterFirstRun))

			g.Expect(s.cleanupVSphere(ctx, folders, resourcePools, folders)).To(gomega.Succeed())
			existingObjects, err = recursiveListFoldersAndResourcePools(ctx, relativePath, clients.Govmomi, clients.Finder, clients.ViewManager)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(existingObjects).To(gomega.BeEquivalentTo(tt.wantAfterSecondRun))

			// Ensure the parent object still exists
			assertObjectExists(ctx, g, clients.Finder, folder)
			assertObjectExists(ctx, g, clients.Finder, resourcePool)
		})
	}
}

func assertObjectExists(ctx context.Context, g *gomega.WithT, finder *find.Finder, inventoryPath string) {
	g.THelper()

	e, err := finder.ManagedObjectList(ctx, inventoryPath)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(e).To(gomega.HaveLen(1))
}

func recursiveListFoldersAndResourcePools(ctx context.Context, testPrefix string, govmomiClient *govmomi.Client, finder *find.Finder, viewManager *view.Manager) (map[string]bool, error) {
	resourcePoolElements, err := recursiveList(ctx, path.Join(resourcePoolBase, testPrefix), govmomiClient, finder, viewManager)
	if err != nil {
		return nil, err
	}

	folderElements, err := recursiveList(ctx, path.Join(folderBase, testPrefix), govmomiClient, finder, viewManager)
	if err != nil {
		return nil, err
	}

	objects := map[string]bool{}

	for _, e := range append(resourcePoolElements, folderElements...) {
		splitted := strings.Split(e.element.Path, testPrefix+"/")
		if len(splitted) == 2 {
			objects[path.Join(e.element.Object.Reference().Type, splitted[1])] = true
		}
	}

	return objects, nil
}

type vcsimObject struct {
	pathSuffix       string
	objectType       string
	datastoreTempDir string
}

func (o vcsimObject) Path(testPrefix string) string {
	var pathPrefix string

	switch o.objectType {
	case "ResourcePool":
		pathPrefix = resourcePoolBase
	case "Folder":
		pathPrefix = folderBase
	case "VirtualMachine":
		// VMs exist at the folders.
		pathPrefix = folderBase
	case "Datastore":
		pathPrefix = "/DC0/datastore"
	default:
		panic("unimplemented")
	}

	return path.Join(pathPrefix, testPrefix, o.pathSuffix)
}

func (o vcsimObject) Create(sim *vcsim.Simulator, testPrefix string) error {
	var cmd string
	switch o.objectType {
	case "ResourcePool":
		cmd = fmt.Sprintf("pool.create %s", o.Path(testPrefix))
	case "Folder":
		cmd = fmt.Sprintf("folder.create %s", o.Path(testPrefix))
	case "Datastore":
		tmpDir, err := os.MkdirTemp(o.datastoreTempDir, testPrefix)
		if err != nil {
			return err
		}
		cmd = fmt.Sprintf("datastore.create -type local -name %s -path %s /DC0/host/DC0_C0", testPrefix, tmpDir)
	case "VirtualMachine":
		fullPath := o.Path(testPrefix)
		folderPath := path.Dir(fullPath)
		rpPath := vcsimResourcePool(path.Dir(o.pathSuffix)).Path(testPrefix)
		name := path.Base(fullPath)
		networkPath := "/DC0/network/DC0_DVPG0"
		cmd = fmt.Sprintf("vm.create -on=true -pool %s -folder %s -net %s -ds /DC0/datastore/%s %s", rpPath, folderPath, networkPath, testPrefix, name)
	default:
		panic("unimplemented")
	}

	stdout, stderr := gbytes.NewBuffer(), gbytes.NewBuffer()
	err := sim.Run(cmd, stdout, stderr)
	if err != nil {
		fmt.Printf("stdout:\n%s\n", stdout.Contents())
		fmt.Printf("stderr:\n%s\n", stderr.Contents())
		return err
	}
	return nil
}

func vcsimResourcePool(p string) *vcsimObject {
	return &vcsimObject{pathSuffix: p, objectType: "ResourcePool"}
}

func vcsimFolder(p string) *vcsimObject {
	return &vcsimObject{pathSuffix: p, objectType: "Folder"}
}

func vcsimDatastore(p, datastoreTempDir string) *vcsimObject {
	return &vcsimObject{pathSuffix: p, objectType: "Datastore", datastoreTempDir: datastoreTempDir}
}

func vcsimVirtualMachine(p string) *vcsimObject {
	return &vcsimObject{pathSuffix: p, objectType: "VirtualMachine"}
}
