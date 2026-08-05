/*
Copyright 2026 The Kubernetes Authors.

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

package replicator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/cluster-api-provider-vsphere/feature"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/template"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

const testNamespace = "test-ns"

// localDatastore mirrors the identically-named helper in the template
// package's own tests: an isolated local datastore on cluster's first host,
// so replicas placed there are provably not shared with another datastore.
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

func newFakeClient() client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

// vmDatastores returns the set of datastore MoRef values vm's files live on.
func vmDatastores(ctx context.Context, g *WithT, vm *object.VirtualMachine) map[string]bool {
	var moVM mo.VirtualMachine
	g.Expect(vm.Properties(ctx, vm.Reference(), []string{"datastore"}, &moVM)).To(Succeed())
	out := make(map[string]bool, len(moVM.Datastore))
	for _, ds := range moVM.Datastore {
		out[ds.Value] = true
	}
	return out
}

func Test_Replicator_RequestReplica(t *testing.T) {
	ctx := context.Background()

	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 2
	model.ClusterHost = 1
	model.Host = 0
	g := NewWithT(t)
	g.Expect(model.Create()).To(Succeed())
	defer model.Remove()

	srv := model.Service.NewServer()
	defer srv.Close()

	c, err := govmomi.NewClient(ctx, srv.URL, true)
	g.Expect(err).ToNot(HaveOccurred())

	finder := find.NewFinder(c.Client)
	dc, err := finder.DefaultDatacenter(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	finder.SetDatacenter(dc)

	datastore0, ds0, cluster0, pool0, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C0")
	g.Expect(err).ToNot(HaveOccurred())
	datastore1, ds1, _, pool1, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C1")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ds0).ToNot(Equal(ds1))
	_ = datastore0

	sess := &session.Session{Client: c, Finder: finder}

	t.Run("fresh replica: clones master into the target datastore and marks it, then releases the lock", func(t *testing.T) {
		g := NewWithT(t)
		const name = "fresh-replica"

		master, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())

		r := &Replicator{Client: newFakeClient(), Namespace: testNamespace}

		g.Expect(r.RequestReplica(ctx, sess, name, master, pool1, datastore1)).To(Succeed())

		// Drive the async state machine forward by re-invoking RequestReplica,
		// exactly as repeated Machine reconciles would.
		g.Eventually(func(g Gomega) {
			g.Expect(r.RequestReplica(ctx, sess, name, master, pool1, datastore1)).To(Succeed())

			var lock corev1.ConfigMap
			err := r.Client.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: lockName(name, datastore1.Reference())}, &lock)
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(err).To(HaveOccurred(), "lock should be deleted once the clone completes")
		}, 10*time.Second, 50*time.Millisecond).Should(Succeed())

		candidates, err := finder.VirtualMachineList(ctx, name)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(candidates).To(HaveLen(2), "master plus the one new replica")

		var replica *object.VirtualMachine
		ds1Ref := datastore1.Reference().Value
		for _, c := range candidates {
			if vmDatastores(ctx, g, c)[ds1Ref] {
				replica = c
			}
		}
		g.Expect(replica).ToNot(BeNil(), "expected a new local replica on datastore1")

		var moReplica mo.VirtualMachine
		g.Expect(replica.Properties(ctx, replica.Reference(), []string{"config"}, &moReplica)).To(Succeed())
		g.Expect(moReplica.Config.Template).To(BeTrue())
	})

	t.Run("concurrent call while a request is in flight is a no-op", func(t *testing.T) {
		g := NewWithT(t)
		const name = "concurrent-request"

		master, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())

		r := &Replicator{Client: newFakeClient(), Namespace: testNamespace}

		g.Expect(r.RequestReplica(ctx, sess, name, master, pool1, datastore1)).To(Succeed())

		lockKey := client.ObjectKey{Namespace: testNamespace, Name: lockName(name, datastore1.Reference())}
		var first corev1.ConfigMap
		g.Expect(r.Client.Get(ctx, lockKey, &first)).To(Succeed())
		firstTask := first.Annotations[annotationTask]
		g.Expect(firstTask).ToNot(BeEmpty())

		// A second, concurrent call (simulating another Machine's reconcile
		// hitting the same missing-replica condition) must not start a
		// second clone task.
		g.Expect(r.RequestReplica(ctx, sess, name, master, pool1, datastore1)).To(Succeed())

		var second corev1.ConfigMap
		g.Expect(r.Client.Get(ctx, lockKey, &second)).To(Succeed())
		g.Expect(second.Annotations[annotationTask]).To(Equal(firstTask), "must not have started a second task")
	})

	t.Run("lost track of an in-flight task: recovers by releasing and retrying, not stuck forever", func(t *testing.T) {
		g := NewWithT(t)
		const name = "lost-task"

		master, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())

		r := &Replicator{Client: newFakeClient(), Namespace: testNamespace}
		lockKey := client.ObjectKey{Namespace: testNamespace, Name: lockName(name, datastore1.Reference())}

		g.Expect(r.RequestReplica(ctx, sess, name, master, pool1, datastore1)).To(Succeed())

		// Simulate vCenter having pruned the task from its history.
		var lock corev1.ConfigMap
		g.Expect(r.Client.Get(ctx, lockKey, &lock)).To(Succeed())
		lock.Annotations[annotationTask] = "task-does-not-exist-9999"
		g.Expect(r.Client.Update(ctx, &lock)).To(Succeed())

		g.Eventually(func(g Gomega) {
			g.Expect(r.RequestReplica(ctx, sess, name, master, pool1, datastore1)).To(Succeed())

			var l corev1.ConfigMap
			err := r.Client.Get(ctx, lockKey, &l)
			g.Expect(err).To(HaveOccurred(), "lock should be deleted once a fresh retry completes")
		}, 10*time.Second, 50*time.Millisecond).Should(Succeed())

		candidates, err := finder.VirtualMachineList(ctx, name)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(candidates).To(HaveLen(2), "master plus exactly one replica from the successful retry")
	})

	t.Run("true concurrent callers never produce more than one replica", func(t *testing.T) {
		g := NewWithT(t)
		const name = "true-concurrency"
		const concurrency = 8

		master, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())

		r := &Replicator{Client: newFakeClient(), Namespace: testNamespace}

		start := make(chan struct{})
		errs := make(chan error, concurrency)
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs <- r.RequestReplica(ctx, sess, name, master, pool1, datastore1)
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			g.Expect(err).ToNot(HaveOccurred())
		}

		g.Eventually(func(g Gomega) {
			g.Expect(r.RequestReplica(ctx, sess, name, master, pool1, datastore1)).To(Succeed())

			var lock corev1.ConfigMap
			err := r.Client.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: lockName(name, datastore1.Reference())}, &lock)
			g.Expect(err).To(HaveOccurred(), "lock should be deleted once the (single) clone completes")
		}, 10*time.Second, 50*time.Millisecond).Should(Succeed())

		candidates, err := finder.VirtualMachineList(ctx, name)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(candidates).To(HaveLen(2), "master plus exactly one replica: concurrency must never produce duplicates")
	})

	t.Run("single-Machine failure domain: lock is still cleaned up even though nobody calls RequestReplica a second time", func(t *testing.T) {
		g := NewWithT(t)
		featuregatetesting.SetFeatureGateDuringTest(t, feature.Gates, feature.TemplateAutoReplicate, true)
		const name = "single-machine-lifecycle"

		master, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
		g.Expect(err).ToNot(HaveOccurred())

		r := &Replicator{Client: newFakeClient(), Namespace: testNamespace}
		lockKey := client.ObjectKey{Namespace: testNamespace, Name: lockName(name, datastore1.Reference())}

		// One Machine's Clone() call: no local replica yet, falls back to
		// master, fires RequestReplica once. It never calls FindTemplate again.
		found, err := template.FindTemplate(ctx, sess, name, pool1, datastore1, r)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found.Reference()).To(Equal(master.Reference()), "falls back to master while the replica clones in the background")

		// Let the background clone finish without ever calling RequestReplica
		// again; nobody in this scenario does.
		g.Eventually(func(g Gomega) {
			var lock corev1.ConfigMap
			g.Expect(r.Client.Get(ctx, lockKey, &lock)).To(Succeed())
			taskID := lock.Annotations[annotationTask]
			g.Expect(taskID).ToNot(BeEmpty())
			state, _, err := r.checkTask(ctx, sess, taskID)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(state).To(Equal(types.TaskInfoStateSuccess))
		}, 10*time.Second, 50*time.Millisecond).Should(Succeed())

		// The lock is still sitting there; nothing has told it the task
		// succeeded yet.
		var existingLock corev1.ConfigMap
		g.Expect(r.Client.Get(ctx, lockKey, &existingLock)).To(Succeed())

		// A later Machine's FindTemplate call discovers the completed
		// replica via the normal fast path, which must trigger ReleaseIfDone.
		found, err = template.FindTemplate(ctx, sess, name, pool1, datastore1, r)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found.Reference()).ToNot(Equal(master.Reference()), "must now resolve to the completed local replica")

		err = r.Client.Get(ctx, lockKey, &corev1.ConfigMap{})
		g.Expect(err).To(HaveOccurred(), "lock must be cleaned up once a FindTemplate call confirms the replica is in place")
	})
}

// Test_Replicator_MultipleFailureDomains reproduces 3 failure domains all
// needing a replica of the same template at once, the actual multi-cluster
// scenario this feature exists for. Every replica shares the master's name,
// so if they all landed in one folder they'd collide; each must land in its
// own per-datastore subfolder instead.
func Test_Replicator_MultipleFailureDomains(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 3
	model.ClusterHost = 1
	model.Host = 0
	g.Expect(model.Create()).To(Succeed())
	defer model.Remove()

	srv := model.Service.NewServer()
	defer srv.Close()

	c, err := govmomi.NewClient(ctx, srv.URL, true)
	g.Expect(err).ToNot(HaveOccurred())

	finder := find.NewFinder(c.Client)
	dc, err := finder.DefaultDatacenter(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	finder.SetDatacenter(dc)

	const name = "multi-failure-domain"
	datastore0, ds0, cluster0, pool0, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C0")
	g.Expect(err).ToNot(HaveOccurred())
	datastore1, _, _, pool1, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C1")
	g.Expect(err).ToNot(HaveOccurred())
	datastore2, _, _, pool2, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C2")
	g.Expect(err).ToNot(HaveOccurred())
	_ = datastore0

	master, err := createTemplateOn(ctx, c.Client, finder, cluster0, pool0, ds0, name)
	g.Expect(err).ToNot(HaveOccurred())

	sess := &session.Session{Client: c, Finder: finder}
	r := &Replicator{Client: newFakeClient(), Namespace: testNamespace}

	for _, target := range []struct {
		pool      *object.ResourcePool
		datastore *object.Datastore
	}{
		{pool1, datastore1},
		{pool2, datastore2},
	} {
		g.Expect(r.RequestReplica(ctx, sess, name, master, target.pool, target.datastore)).To(Succeed())
		lockKey := client.ObjectKey{Namespace: testNamespace, Name: lockName(name, target.datastore.Reference())}
		g.Eventually(func(g Gomega) {
			g.Expect(r.RequestReplica(ctx, sess, name, master, target.pool, target.datastore)).To(Succeed())
			err := r.Client.Get(ctx, lockKey, &corev1.ConfigMap{})
			g.Expect(err).To(HaveOccurred(), "lock should be deleted once this datastore's clone completes")
		}, 10*time.Second, 50*time.Millisecond).Should(Succeed())
	}

	candidates, err := finder.VirtualMachineList(ctx, name)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(candidates).To(HaveLen(3), "master plus one replica per failure domain, no collisions")
}

// Test_Replicator_SameClusterMultipleDatastores reproduces a single cluster
// with two datastores that both need their own local replica, the exact
// case the user flagged: a cluster commonly has several datastores, and
// different Machine clones in that same failure domain (one pinned via
// Spec.Datastore, another landing elsewhere via storage-policy selection)
// can each need a replica on a different one. Keying the lock/folder by
// cluster instead of datastore would collapse these into one shared
// replica, and the second RequestReplica call would either be a permanent
// no-op (locked out by the first's already-satisfied lock) or collide on a
// shared folder.
func Test_Replicator_SameClusterMultipleDatastores(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	model := simulator.VPX()
	model.Datacenter = 1
	model.Cluster = 1
	model.ClusterHost = 1
	model.Host = 0
	g.Expect(model.Create()).To(Succeed())
	defer model.Remove()

	srv := model.Service.NewServer()
	defer srv.Close()

	c, err := govmomi.NewClient(ctx, srv.URL, true)
	g.Expect(err).ToNot(HaveOccurred())

	finder := find.NewFinder(c.Client)
	dc, err := finder.DefaultDatacenter(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	finder.SetDatacenter(dc)

	const name = "same-cluster-multi-datastore"
	cluster, err := finder.ClusterComputeResource(ctx, "/DC0/host/DC0_C0")
	g.Expect(err).ToNot(HaveOccurred())
	pool, err := cluster.ResourcePool(ctx)
	g.Expect(err).ToNot(HaveOccurred())

	// The master lives on the cluster's built-in default datastore. Two more
	// datastores, A and B, are attached to that SAME cluster/pool; neither
	// holds the master. Mirrors two Machine clones in the same failure
	// domain that land on different datastores and each need their own
	// local replica.
	defaultDS, err := finder.DefaultDatastore(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	master, err := createTemplateOn(ctx, c.Client, finder, cluster, pool, defaultDS.Name(), name)
	g.Expect(err).ToNot(HaveOccurred())

	datastoreA, _, _, _, err := localDatastore(ctx, model, finder, "/DC0/host/DC0_C0")
	g.Expect(err).ToNot(HaveOccurred())
	datastoreB, err := secondLocalDatastore(ctx, model, dc, cluster, "local-ds-b")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(datastoreA.Reference()).ToNot(Equal(datastoreB.Reference()))

	sess := &session.Session{Client: c, Finder: finder}
	r := &Replicator{Client: newFakeClient(), Namespace: testNamespace}

	for _, datastore := range []*object.Datastore{datastoreA, datastoreB} {
		g.Expect(r.RequestReplica(ctx, sess, name, master, pool, datastore)).To(Succeed())
		lockKey := client.ObjectKey{Namespace: testNamespace, Name: lockName(name, datastore.Reference())}
		g.Eventually(func(g Gomega) {
			g.Expect(r.RequestReplica(ctx, sess, name, master, pool, datastore)).To(Succeed())
			err := r.Client.Get(ctx, lockKey, &corev1.ConfigMap{})
			g.Expect(err).To(HaveOccurred(), "lock should be deleted once this datastore's clone completes")
		}, 10*time.Second, 50*time.Millisecond).Should(Succeed())
	}

	candidates, err := finder.VirtualMachineList(ctx, name)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(candidates).To(HaveLen(3), "master plus one independent replica per datastore, all in the same cluster, no collision")

	for _, datastore := range []*object.Datastore{datastoreA, datastoreB} {
		dsRef := datastore.Reference().Value
		var found bool
		for _, cand := range candidates {
			if vmDatastores(ctx, g, cand)[dsRef] {
				found = true
			}
		}
		g.Expect(found).To(BeTrue(), "expected a replica specifically on this datastore, not just anywhere in the cluster")
	}
}

// secondLocalDatastore creates one more isolated local datastore on
// cluster's first host, for tests that already have a first one from
// localDatastore and need a second, differently-named one on the same
// cluster.
func secondLocalDatastore(ctx context.Context, model *simulator.Model, dc *object.Datacenter, cluster *object.ClusterComputeResource, dsName string) (*object.Datastore, error) {
	hosts, err := cluster.Hosts(ctx)
	if err != nil || len(hosts) == 0 {
		return nil, fmt.Errorf("listing hosts for cluster: %w", err)
	}
	dss, err := hosts[0].ConfigManager().DatastoreSystem(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting datastore system: %w", err)
	}
	dsDir, err := os.MkdirTemp("", "vcsim-local-ds-")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir for local datastore: %w", err)
	}
	ds, err := dss.CreateLocalDatastore(ctx, dsName, dsDir)
	if err != nil {
		return nil, fmt.Errorf("creating local datastore: %w", err)
	}
	simCtx := model.Service.Context
	dcObj, ok := simCtx.Map.Get(dc.Reference()).(*simulator.Datacenter)
	if !ok {
		return nil, fmt.Errorf("unexpected type for datacenter object in simulator registry")
	}
	simCtx.Map.AddReference(simCtx, dcObj, &dcObj.Datastore, ds.Reference())
	return ds, nil
}
