/*
Copyright 2022 The Kubernetes Authors.

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

package find_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi/simulator"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/internal/test/helpers/vcsim"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/find"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

const TestHostGroup = "host-group-alpha"

type resourceCount struct {
	datacenters                 int
	clustersInDefaultDatacenter int
	hostsInTestGroup            int
}

func TestObjectFunc(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	sim, authSession, resources, err := setupSimulatorAndSession(simulator.VPX())
	g.Expect(err).ToNot(HaveOccurred(), "a vcsim instance and authSession should be established")
	t.Cleanup(sim.Destroy)

	defaultDatacenter, err := authSession.Finder.DefaultDatacenter(ctx)
	g.Expect(err).ToNot(HaveOccurred())

	topology := infrav1.Topology{
		Datacenter:     defaultDatacenter.Name(),
		ComputeCluster: ptr.To(defaultDatacenter.Name() + "_C0"),
		Hosts:          &infrav1.FailureDomainHosts{HostGroupName: TestHostGroup},
	}

	testCases := []struct {
		name          string
		failureDomain infrav1.FailureDomainType
		topology      infrav1.Topology
		testFunc      func(*WithT, find.ManagedRefFinder)
	}{
		{
			name:          "get and test a finder for compute clusters in the default datacenter",
			failureDomain: infrav1.ComputeClusterFailureDomain,
			topology:      topology,
			testFunc: func(g *WithT, refFinder find.ManagedRefFinder) {
				computeClusters, err := refFinder(ctx)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(computeClusters).To(HaveLen(resources.clustersInDefaultDatacenter))
			},
		},
		{
			name:          "get and test a finder for datacenters",
			failureDomain: infrav1.DatacenterFailureDomain,
			topology:      topology,
			testFunc: func(g *WithT, refFinder find.ManagedRefFinder) {
				datacenters, err := refFinder(ctx)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(datacenters).To(HaveLen(resources.datacenters))
			},
		},
		{
			name:          "get and test a finder for hosts in a host group",
			failureDomain: infrav1.HostGroupFailureDomain,
			topology:      topology,
			testFunc: func(g *WithT, refFinder find.ManagedRefFinder) {
				hostGroups, err := refFinder(ctx)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(hostGroups).To(HaveLen(resources.hostsInTestGroup))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)

			refFinder := find.ObjectFunc(testCase.failureDomain, testCase.topology, authSession.Finder)
			g.Expect(refFinder).ToNot(BeNil())

			testCase.testFunc(g, refFinder)
		})
	}
}

// setupSimulatorAndSession sets up a VC Simulator and a session.Session that connects to it.
// The function also returns a resourceCount instance as a convenient reference of resource counts to test against.
func setupSimulatorAndSession(model *simulator.Model) (*vcsim.Simulator, *session.Session, resourceCount, error) {
	setupCommands := []string{
		fmt.Sprintf("cluster.group.create -name %s -cluster DC0_C0 -host DC0_C0_H0 DC0_C0_H1", TestHostGroup),
	}

	sim, err := vcsim.NewBuilder().WithOperations(setupCommands...).WithModel(model).Build()
	if err != nil {
		return sim, nil, resourceCount{}, err
	}

	// Get a datacenter to use with the finder
	datacenterName := func() string {
		stdout := gbytes.NewBuffer()

		err := sim.Run("find / -type d", stdout)
		if err != nil {
			return ""
		}

		scanner := bufio.NewScanner(stdout)
		scanner.Scan()

		return scanner.Text()
	}()

	authSession, err := session.GetOrCreate(context.Background(),
		session.NewParams().
			WithServer(sim.ServerURL().Host).
			WithUserInfo(sim.Username(), sim.Password()).
			WithDatacenter(datacenterName),
	)
	if err != nil {
		return sim, nil, resourceCount{}, err
	}

	resources, err := countResources(sim, authSession)

	return sim, authSession, resources, err
}

// countResources counts resources relevant to testing find.ObjectFunc into a resourceCount struct.
func countResources(sim *vcsim.Simulator, session *session.Session) (resources resourceCount, err error) {
	defaultDc, err := session.Finder.DefaultDatacenter(context.Background())
	if err != nil {
		return resources, err
	}

	dcPath := defaultDc.InventoryPath

	stdout := gbytes.NewBuffer()

	err = sim.Run("find / -type d", stdout)
	if err != nil {
		return resources, err
	}

	resources.datacenters = countLines(stdout)
	_ = stdout.Clear()

	err = sim.Run(fmt.Sprintf("find %s -type c", dcPath), stdout)
	if err != nil {
		return resources, err
	}

	resources.clustersInDefaultDatacenter = countLines(stdout)
	_ = stdout.Clear()

	err = sim.Run(fmt.Sprintf("cluster.group.ls -l -name %s", TestHostGroup), stdout)
	if err != nil {
		return resources, err
	}

	resources.hostsInTestGroup = countLines(stdout)
	_ = stdout.Clear()

	return resources, nil
}

// countLines counts lines in a reader's buffer.
func countLines(reader io.Reader) int {
	sc := bufio.NewScanner(reader)
	count := 0

	for sc.Scan() {
		count++
	}

	return count
}
