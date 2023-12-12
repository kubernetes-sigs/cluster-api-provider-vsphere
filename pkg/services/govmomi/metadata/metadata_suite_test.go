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

package metadata

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context/fake"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	"sigs.k8s.io/cluster-api-provider-vsphere/test/helpers/vcsim"
)

func TestMetadata(t *testing.T) {
	RegisterFailHandler(Fail)

	reporterConfig := types.NewDefaultReporterConfig()
	if artifactFolder, exists := os.LookupEnv("ARTIFACTS"); exists {
		reporterConfig.JUnitReport = filepath.Join(artifactFolder, "junit.ginkgo.pkg_services_govmomi_metadata.xml")
	}
	RunSpecs(t, "Metadata Suite", reporterConfig)
}

const (
	existingCategory             = "existingCategory"
	existingCategoryAssocType    = infrav1.DatacenterFailureDomain
	existingCategoryNewAssocType = infrav1.HostGroupFailureDomain
	existingTag                  = "existingTag"
)

var (
	sim                *vcsim.Simulator
	vmCtx              *capvcontext.VMContext
	ctx                = context.Background()
	existingCategoryID string
)

var _ = BeforeSuite(func() {
	Expect(configureSimulatorAndContext(ctx)).To(Succeed())
	Expect(createTagsAndCategories()).To(Succeed())
})

var _ = AfterSuite(func() {
	sim.Destroy()
})

var _ = Describe("Metadata_CreateCategory", func() {
	const (
		testCategory          = "testCategory"
		testCategoryAssocType = infrav1.HostGroupFailureDomain
	)

	Context("we attempt to create a new category", func() {
		It("creates a matching category in the context's TagManager", func() {
			catID, err := CreateCategory(ctx, vmCtx, testCategory, testCategoryAssocType)
			Expect(err).ToNot(HaveOccurred())
			Expect(catID).NotTo(BeEmpty())

			cat, err := vmCtx.GetSession().TagManager.GetCategory(ctx, catID)
			Expect(err).ToNot(HaveOccurred())
			Expect(cat.Name).To(Equal(testCategory))
			Expect(cat.AssociableTypes).To(ConsistOf(categoryAssociableTypes()[testCategoryAssocType]))
		})
	})

	Context("we attempt to update an existing category", func() {
		It("updates the matching category in the context's TagManager", func() {
			if existingCategoryID == "" {
				Fail("category required to run this test has not been setup in the VCSim!")
			}

			catID, err := CreateCategory(ctx, vmCtx, existingCategory, existingCategoryNewAssocType)
			Expect(err).ToNot(HaveOccurred())
			Expect(catID).To(Equal(existingCategoryID))

			cat, err := vmCtx.GetSession().TagManager.GetCategory(ctx, catID)
			Expect(err).ToNot(HaveOccurred())
			Expect(cat.Name).To(Equal(existingCategory))
			Expect(cat.AssociableTypes).To(ConsistOf(
				categoryAssociableTypes()[existingCategoryAssocType],
				categoryAssociableTypes()[existingCategoryNewAssocType],
			))
		})
	})
})

var _ = Describe("Metadata_CreateTag", func() {
	const (
		testTag        = "testTag"
		mockCategoryID = "mockCategory"
	)

	BeforeEach(func() {
		if existingCategoryID == "" {
			Fail("category and tags required to run this test have not been setup in the VCSim!")
		}
	})

	Context("we attempt to create a new tag", func() {
		It("creates a tag in the context's TagManager", func() {
			Expect(CreateTag(ctx, vmCtx, testTag, existingCategoryID)).To(Succeed())

			tag, err := vmCtx.GetSession().TagManager.GetTag(ctx, testTag)
			Expect(err).ToNot(HaveOccurred())
			Expect(tag.CategoryID).To(Equal(existingCategoryID))
		})
	})

	Context("we attempt to create a tag which already exists but for a different category ID", func() {
		It("does not return an error and does not modify the existing tag", func() {
			Expect(CreateTag(ctx, vmCtx, existingTag, mockCategoryID)).To(Succeed())

			tag, err := vmCtx.GetSession().TagManager.GetTag(ctx, existingTag)
			Expect(err).ToNot(HaveOccurred())
			Expect(tag.CategoryID).To(Equal(existingCategoryID), "the tag's category ID must not change")
		})
	})
})

func configureSimulatorAndContext(ctx context.Context) (err error) {
	sim, err = vcsim.NewBuilder().Build()
	if err != nil {
		return
	}

	vmCtx = fake.NewVMContext(ctx, fake.NewControllerManagerContext())
	vmCtx.VSphereVM.Spec.Server = sim.ServerURL().Host

	authSession, err := session.GetOrCreate(
		ctx,
		session.NewParams().
			WithServer(vmCtx.VSphereVM.Spec.Server).
			WithUserInfo(sim.Username(), sim.Password()).
			WithDatacenter("*"))

	vmCtx.Session = authSession

	return
}

// createTagsAndCategories creates tag(s) and category(s) required to run the tests on the VCSim.
func createTagsAndCategories() (err error) {
	stdout := gbytes.NewBuffer()
	scanner := bufio.NewScanner(stdout)

	// create a category to support tests for both CreateCategory() and CreateTag()
	err = sim.Run(fmt.Sprintf("tags.category.create -t %s %s",
		categoryAssociableTypes()[existingCategoryAssocType],
		existingCategory),
		stdout,
	)
	if err != nil {
		return
	}

	if scanned := scanner.Scan(); !scanned {
		err = errors.New("failed to retrieve category ID from govc stdout buffer")
		return
	}

	existingCategoryID = scanner.Text()

	// create a tag to support tests for CreateTag()
	err = sim.Run(fmt.Sprintf("tags.create -c %s %s", existingCategory, existingTag))
	return
}

// categoryAssociableTypes() maps failure domain types to resource type strings
// We use this function instead of getCategoryAssociableTypes() to avoid a hard dependency on a package private function.
func categoryAssociableTypes() map[infrav1.FailureDomainType]string {
	return map[infrav1.FailureDomainType]string{
		infrav1.DatacenterFailureDomain:     "Datacenter",
		infrav1.ComputeClusterFailureDomain: "ClusterComputeResource",
		infrav1.HostGroupFailureDomain:      "HostSystem",
	}
}
