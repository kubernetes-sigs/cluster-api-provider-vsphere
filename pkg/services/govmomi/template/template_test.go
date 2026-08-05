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

package template

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

// localDatastore creates an isolated local datastore on the given cluster's
// first host and returns it (both as an *object.Datastore and its name), the
// cluster, and its (default) resource pool. Used to reproduce a topology
// where each failure domain has its own storage that cannot be shared with
// any other failure domain.
func localDatastore(ctx context.Context, model *simulator.Model, finder *find.Finder, clusterPath string) (ds *object.Datastore, dsName string, cluster *object.ClusterComputeResource, pool *object.ResourcePool, err error) {
	cluster, err = finder.ClusterComputeResource(ctx, clusterPath)
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("finding cluster %q: %w", clusterPath, err)
	}
	hosts, err := cluster.Hosts(ctx)
	if err != nil || len(hosts) == 0 {
		return nil, "", nil, nil, fmt.Errorf("listing hosts for cluster %q: %w", clusterPath, err)
	}
	host := hosts[0]

	dss, err := host.ConfigManager().DatastoreSystem(ctx)
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("getting datastore system: %w", err)
	}
	dsName = "local-ds-" + strings.ReplaceAll(strings.Trim(clusterPath, "/"), "/", "-")
	dsDir, err := os.MkdirTemp("", "vcsim-local-ds-")
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("creating temp dir for local datastore: %w", err)
	}
	ds, err = dss.CreateLocalDatastore(ctx, dsName, dsDir)
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("creating local datastore on %q: %w", clusterPath, err)
	}

	// vcsim's HostDatastoreSystem.add() registers the new datastore onto the
	// owning ComputeResource (cluster) but NOT onto the Datacenter's own
	// Datastore list, and Folder.CreateVM resolves "[dsName]" against the
	// latter. Patch it in directly via the simulator's internal registry
	// (only possible/needed because this is an in-process test).
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("getting datacenter: %w", err)
	}
	simCtx := model.Service.Context
	dcObj, ok := simCtx.Map.Get(dc.Reference()).(*simulator.Datacenter)
	if !ok {
		return nil, "", nil, nil, fmt.Errorf("unexpected type for datacenter object in simulator registry")
	}
	simCtx.Map.AddReference(simCtx, dcObj, &dcObj.Datastore, ds.Reference())

	pool, err = cluster.ResourcePool(ctx)
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("getting resource pool for %q: %w", clusterPath, err)
	}
	return ds, dsName, cluster, pool, nil
}

// createTemplateOn creates a powered-off VM named templateName on dsName and
// marks it as a template.
func createTemplateOn(ctx context.Context, client *vim25.Client, finder *find.Finder, cluster *object.ClusterComputeResource, pool *object.ResourcePool, dsName, templateName string) (*object.VirtualMachine, error) {
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting datacenter: %w", err)
	}
	folders, err := dc.Folders(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting datacenter folders: %w", err)
	}
	hosts, err := cluster.Hosts(ctx)
	if err != nil || len(hosts) == 0 {
		return nil, fmt.Errorf("listing hosts for cluster: %w", err)
	}

	spec := types.VirtualMachineConfigSpec{
		Name: templateName,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: fmt.Sprintf("[%s]", dsName),
		},
		NumCPUs:  1,
		MemoryMB: 128,
	}
	task, err := folders.VmFolder.CreateVM(ctx, spec, pool, hosts[0])
	if err != nil {
		return nil, fmt.Errorf("creating vm %q on datastore %q: %w", templateName, dsName, err)
	}
	res, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("waiting for vm creation %q on datastore %q: %w", templateName, dsName, err)
	}
	vm := object.NewVirtualMachine(client, res.Result.(types.ManagedObjectReference))
	if err := vm.MarkAsTemplate(ctx); err != nil {
		return nil, fmt.Errorf("marking %q as template on datastore %q: %w", templateName, dsName, err)
	}
	return vm, nil
}

// Test_FindTemplate_LocalityPreference reproduces a cross-cluster-HA
// topology where each failure domain has its own isolated local datastore,
// and asserts FindTemplate's behavior across every local/global match count
// with TemplateAutoReplicate enabled, plus the gate-disabled baseline.
func Test_FindTemplate_LocalityPreference(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 2
	model.ClusterHost = 1
	model.Host = 0
	// Leave the VPX default (Datastore: 1) for the model's own default VMs to
	// land on; we attach an additional, isolated local datastore per cluster
	// below for our own test templates.
	g.Expect(model.Create()).To(Succeed())
	defer model.Remove()

	srv := model.Service.NewServer()
	defer srv.Close()

	// Establish a real, authenticated govmomi client against the simulator
	// (srv.URL already carries the simulator's generated credentials).
	c, err := govmomi.NewClient(ctx, srv.URL, true)
	g.Expect(err).ToNot(HaveOccurred())

	finder := find.NewFinder(c.Client)
	dc, err := finder.DefaultDatacenter(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	finder.SetDatacenter(dc)

	datastore0, ds0, cluster0, pool0, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C0")
	g.Expect(err).ToNot(HaveOccurred())
	datastore1, ds1, cluster1, pool1, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C1")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ds0).ToNot(Equal(ds1), "the two clusters must have different, isolated datastores")

	sess := &session.Session{
		Client: c,
		Finder: finder,
	}

	t.Run("exactly one local match: used directly, per datastore", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.TemplateAutoReplicate, true)
		const name = "replicated-template"

		vm0, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())
		vm1, err := createTemplateOn(ctx, c.Client, finder, cluster1, pool1, ds1, name)
		g.Expect(err).ToNot(HaveOccurred())

		found, err := FindTemplate(ctx, sess, name, pool0, datastore0, nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found.Reference()).To(Equal(vm0.Reference()))

		found, err = FindTemplate(ctx, sess, name, pool1, datastore1, nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found.Reference()).To(Equal(vm1.Reference()))
	})

	t.Run("more than one global match, no datastore given: same ambiguity error as today", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.TemplateAutoReplicate, true)
		const name = "replicated-template-no-pool"

		_, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())
		_, err = createTemplateOn(ctx, c.Client, finder, cluster1, pool1, ds1, name)
		g.Expect(err).ToNot(HaveOccurred())

		_, err = FindTemplate(ctx, sess, name, nil, nil, nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err).To(MatchError(ContainSubstring("unable to find template by name")))
	})

	t.Run("zero local matches, exactly one global match: falls back to it while a copy clones in the background", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.TemplateAutoReplicate, true)
		const name = "single-template-not-on-cluster1"

		vm0, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())

		// datastore1 has no local copy of this template; must still resolve
		// to the one that exists globally.
		found, err := FindTemplate(ctx, sess, name, pool1, datastore1, nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found.Reference()).To(Equal(vm0.Reference()))
	})

	t.Run("more than one local match: same ambiguity error as today, not a silent guess", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.TemplateAutoReplicate, true)
		const name = "colliding-template"

		// Two independent VMs, same name, both genuinely local to
		// datastore0: a real misconfiguration, not the intended
		// one-copy-per-datastore pattern.
		_, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())
		_, err = createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())

		_, err = FindTemplate(ctx, sess, name, pool0, datastore0, nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err).To(MatchError(ContainSubstring("unable to find template by name")))
	})

	t.Run("gate disabled: no locality preference, same behavior as before this feature existed", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.TemplateAutoReplicate, false)
		const name = "gate-off-template"

		// Two same-named VMs across datastores: with the gate on, one of
		// these would resolve cleanly via locality preference. With it off,
		// this must error exactly as the original single-result Finder call
		// always has.
		_, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())
		_, err = createTemplateOn(ctx, c.Client, finder, cluster1, pool1, ds1, name)
		g.Expect(err).ToNot(HaveOccurred())

		_, err = FindTemplate(ctx, sess, name, pool0, datastore0, nil)
		g.Expect(err).To(HaveOccurred(), "with the gate off, template resolution must be unaware of datastore locality")
	})
}
