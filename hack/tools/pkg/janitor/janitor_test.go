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

package janitor

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/cns"
	cnssimulator "github.com/vmware/govmomi/cns/simulator"
	cnstypes "github.com/vmware/govmomi/cns/types"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/simulator/vpx"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
)

func setup(ctx context.Context, t *testing.T) (*VSphereClients, *vcsim.Simulator) {
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

	model.Service.RegisterSDK(cnssimulator.New())

	fmt.Printf(" export GOVC_URL=%s\n", vcsim.ServerURL())
	fmt.Printf(" export GOVC_USERNAME=%s\n", vcsim.Username())
	fmt.Printf(" export GOVC_PASSWORD=%s\n", vcsim.Password())
	fmt.Printf(" export GOVC_INSECURE=true\n")

	clients, err := NewVSphereClients(ctx, NewVSphereClientsInput{
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

func setupTestCase(ctx context.Context, g *gomega.WithT, sim *vcsim.Simulator, clients *VSphereClients, objects []vcsimObject) string {
	g.THelper()

	relativePath := rand.String(10)

	baseRP := vcsimResourcePool("")
	baseFolder := vcsimFolder("")
	baseDatastore := vcsimDatastore("", os.TempDir())
	// Create base objects for the test case
	g.Expect(baseRP.Create(ctx, sim, clients, relativePath)).To(gomega.Succeed())
	g.Expect(baseFolder.Create(ctx, sim, clients, relativePath)).To(gomega.Succeed())
	g.Expect(baseDatastore.Create(ctx, sim, clients, relativePath)).To(gomega.Succeed())

	// Create objects
	for _, object := range objects {
		g.Expect(object.Create(ctx, sim, clients, relativePath)).To(gomega.Succeed())
	}

	return relativePath
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
	defer sim.Destroy()

	tests := []struct {
		name    string
		objects []vcsimObject
		wantErr bool
		want    map[string]bool
	}{
		{
			name: "delete all VMs",
			objects: []vcsimObject{
				vcsimVirtualMachine("foo"),
			},
			wantErr: false,
			want:    nil,
		},
		{
			name: "recursive vm deletion",
			objects: []vcsimObject{
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
			wantErr: false,
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

			relativePath := setupTestCase(ctx, g, sim, clients, tt.objects)

			s := &Janitor{
				dryRun:         false,
				vSphereClients: clients,
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
	defer sim.Destroy()

	tests := []struct {
		name       string
		basePath   string
		objectType string
		objects    []vcsimObject
		wantErr    bool
		want       map[string]bool
	}{
		{
			name:       "should preserve resource pool if it contains a vm and delete empty resource pools",
			basePath:   resourcePoolBase,
			objectType: "ResourcePool",
			objects: []vcsimObject{
				vcsimResourcePool("a"),
				vcsimResourcePool("b"), // this one will be deleted
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
			objects: []vcsimObject{
				vcsimResourcePool("a"),
				vcsimFolder("a"),
				vcsimFolder("b"), // this one will be deleted
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
			objects:    []vcsimObject{},
		},
		{
			name:       "single resource pool",
			basePath:   resourcePoolBase,
			objectType: "ResourcePool",
			objects: []vcsimObject{
				vcsimResourcePool("a"),
			},
		},
		{
			name:       "multiple nested resource pools",
			basePath:   resourcePoolBase,
			objectType: "ResourcePool",
			objects: []vcsimObject{
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
			objects:    []vcsimObject{},
		},
		{
			name:       "single folder",
			basePath:   folderBase,
			objectType: "Folder",
			objects: []vcsimObject{
				vcsimFolder("a"),
			},
		},
		{
			name:       "multiple nested folders",
			basePath:   folderBase,
			objectType: "Folder",
			objects: []vcsimObject{
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

			relativePath := setupTestCase(ctx, g, sim, clients, tt.objects)

			inventoryPath := path.Join(tt.basePath, relativePath)

			s := &Janitor{
				dryRun:         false,
				vSphereClients: clients,
			}

			g.Expect(s.deleteObjectChildren(ctx, inventoryPath, tt.objectType)).To(gomega.Succeed())
			existingObjects, err := recursiveListFoldersAndResourcePools(ctx, relativePath, clients.Govmomi, clients.Finder, clients.ViewManager)
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

func TestJanitor_deleteCNSVolumes(t *testing.T) {
	ctx := context.Background()
	ctx = ctrl.LoggerInto(ctx, klog.Background())

	// Initialize and start vcsim
	clients, sim := setup(ctx, t)
	defer sim.Destroy()

	_ = sim
	tests := []struct {
		name        string
		objects     []vcsimObject
		wantVolumes int
	}{
		{
			name:        "noop",
			objects:     []vcsimObject{},
			wantVolumes: 0,
		},
		{
			name: "Keep other volumes",
			objects: []vcsimObject{
				vcsimCNSVolume("this", true),
				vcsimCNSVolume("this", true),
				vcsimCNSVolume("other", true),
			},
			wantVolumes: 1,
		},
		{
			name: "Ignore volume without PVC metadata",
			objects: []vcsimObject{
				vcsimCNSVolume("this", true),
				vcsimCNSVolume("this", true),
				vcsimCNSVolume("this", false),
			},
			wantVolumes: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			s := &Janitor{
				dryRun:         false,
				vSphereClients: clients,
			}

			relativePath := setupTestCase(ctx, g, sim, clients, tt.objects)

			boskosResource := relativePath + "-this"

			// Check that all volumes exist.
			cnsVolumes, err := queryTestCNSVolumes(ctx, clients.CNS, relativePath)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(cnsVolumes).To(gomega.HaveLen(len(tt.objects)))

			// Run deletion but only for the given boskosResource.
			g.Expect(s.DeleteCNSVolumes(ctx, boskosResource)).To(gomega.Succeed())

			// Check that the expected number of volumes are preserved.
			cnsVolumes, err = queryTestCNSVolumes(ctx, clients.CNS, relativePath)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(cnsVolumes).To(gomega.HaveLen(tt.wantVolumes))
		})
	}
}

func Test_janitor_CleanupVSphere(t *testing.T) {
	ctx := context.Background()
	ctx = ctrl.LoggerInto(ctx, klog.Background())

	// Initialize and start vcsim
	clients, sim := setup(ctx, t)
	defer sim.Destroy()

	tests := []struct {
		name        string
		dryRun      bool
		objects     []vcsimObject
		want        map[string]bool
		wantVolumes int
	}{
		{
			name:        "no-op",
			dryRun:      false,
			objects:     nil,
			want:        map[string]bool{},
			wantVolumes: 0,
		},
		{
			name:        "dryRun: no-op",
			dryRun:      true,
			objects:     nil,
			want:        map[string]bool{},
			wantVolumes: 0,
		},
		{
			name:   "delete everything",
			dryRun: false,
			objects: []vcsimObject{
				vcsimFolder("a"),
				vcsimResourcePool("a"),
				vcsimVirtualMachine("a/b"),
				vcsimFolder("c"),
				vcsimResourcePool("c"),
				vcsimCNSVolume("this", true),
				vcsimCNSVolume("other", true),
			},
			want:        map[string]bool{},
			wantVolumes: 1,
		},
		{
			name:   "dryRun: would delete everything",
			dryRun: true,
			objects: []vcsimObject{
				vcsimFolder("a"),
				vcsimResourcePool("a"),
				vcsimVirtualMachine("a/b"),
				vcsimFolder("c"),
				vcsimResourcePool("c"),
				vcsimCNSVolume("this", true),
			},
			want: map[string]bool{
				"Folder/a":           true,
				"Folder/c":           true,
				"ResourcePool/a":     true,
				"ResourcePool/c":     true,
				"VirtualMachine/a/b": true,
			},
			wantVolumes: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			relativePath := setupTestCase(ctx, g, sim, clients, tt.objects)

			s := &Janitor{
				dryRun:         tt.dryRun,
				vSphereClients: clients,
			}

			boskosResource := relativePath + "-this"

			folder := vcsimFolder("").Path(relativePath)
			resourcePool := vcsimResourcePool("").Path(relativePath)

			folders := []string{folder}
			resourcePools := []string{resourcePool}

			g.Expect(s.CleanupVSphere(ctx, folders, resourcePools, folders, boskosResource, false)).To(gomega.Succeed())
			existingObjects, err := recursiveListFoldersAndResourcePools(ctx, relativePath, clients.Govmomi, clients.Finder, clients.ViewManager)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(existingObjects).To(gomega.BeEquivalentTo(tt.want))

			// Ensure the parent object still exists
			assertObjectExists(ctx, g, clients.Finder, folder)
			assertObjectExists(ctx, g, clients.Finder, resourcePool)

			cnsVolumes, err := queryTestCNSVolumes(ctx, clients.CNS, relativePath)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(cnsVolumes).To(gomega.HaveLen(tt.wantVolumes))
		})
	}
}

func queryTestCNSVolumes(ctx context.Context, client *cns.Client, testPrefix string) ([]cnstypes.CnsVolume, error) {
	// VCSim only implements queryfilters on volume IDs.
	res, err := client.QueryVolume(ctx, cnstypes.CnsQueryFilter{})
	if err != nil {
		return nil, err
	}

	volumes := []cnstypes.CnsVolume{}

	for _, volume := range res.Volumes {
		if strings.HasPrefix(volume.Name, testPrefix) {
			volumes = append(volumes, volume)
		}
	}

	return volumes, nil
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

type vcsimObject interface {
	Create(ctx context.Context, sim *vcsim.Simulator, vsphereClients *VSphereClients, testPrefix string) error
}

type vcsimInventoryObject struct {
	pathSuffix       string
	objectType       string
	datastoreTempDir string
}

func (o vcsimInventoryObject) Path(testPrefix string) string {
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

func (o vcsimInventoryObject) Create(_ context.Context, sim *vcsim.Simulator, _ *VSphereClients, testPrefix string) error {
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

type vcsimCNSVolumeObject struct {
	boskosResourceName string
	hasPVCMetadata     bool
}

func (v vcsimCNSVolumeObject) Create(ctx context.Context, _ *vcsim.Simulator, vsphereClients *VSphereClients, testPrefix string) error {
	ds, err := vsphereClients.Finder.Datastore(ctx, testPrefix)
	if err != nil {
		return err
	}

	spec := cnstypes.CnsVolumeCreateSpec{
		Name:       fmt.Sprintf("%s-pvc-%s", testPrefix, uuid.New().String()),
		VolumeType: string(cnstypes.CnsVolumeTypeBlock),
		Datastores: []types.ManagedObjectReference{ds.Reference()},
		Metadata: cnstypes.CnsVolumeMetadata{
			EntityMetadata: []cnstypes.BaseCnsEntityMetadata{},
		},
		BackingObjectDetails: &cnstypes.CnsBlockBackingDetails{
			CnsBackingObjectDetails: cnstypes.CnsBackingObjectDetails{
				CapacityInMb: 5120,
			},
		},
	}

	if v.hasPVCMetadata {
		spec.Metadata.EntityMetadata = append(spec.Metadata.EntityMetadata, &cnstypes.CnsKubernetesEntityMetadata{
			EntityType: string(cnstypes.CnsKubernetesEntityTypePVC),
			CnsEntityMetadata: cnstypes.CnsEntityMetadata{
				Labels: []types.KeyValue{
					{
						Key:   boskosResourceLabel,
						Value: testPrefix + "-" + v.boskosResourceName,
					},
				},
			},
		})
	}

	task, err := vsphereClients.CNS.CreateVolume(ctx, []cnstypes.CnsVolumeCreateSpec{spec})
	if err != nil {
		return err
	}

	return waitForTasksFinished(ctx, []*object.Task{task}, false)
}

func vcsimResourcePool(p string) *vcsimInventoryObject {
	return &vcsimInventoryObject{pathSuffix: p, objectType: "ResourcePool"}
}

func vcsimFolder(p string) *vcsimInventoryObject {
	return &vcsimInventoryObject{pathSuffix: p, objectType: "Folder"}
}

func vcsimDatastore(p, datastoreTempDir string) *vcsimInventoryObject {
	return &vcsimInventoryObject{pathSuffix: p, objectType: "Datastore", datastoreTempDir: datastoreTempDir}
}

func vcsimVirtualMachine(p string) *vcsimInventoryObject {
	return &vcsimInventoryObject{pathSuffix: p, objectType: "VirtualMachine"}
}

func vcsimCNSVolume(boskosResourceName string, hasPVCMetadata bool) *vcsimCNSVolumeObject {
	return &vcsimCNSVolumeObject{boskosResourceName: boskosResourceName, hasPVCMetadata: hasPVCMetadata}
}
