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

package feature

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	vmoprv1alpha2 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	vmoprv1alpha6 "github.com/vmware-tanzu/vm-operator/api/v1alpha6"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	CommonFeature                      featuregate.Feature = "CommonFeature"
	CommonFeatureTrue                  featuregate.Feature = "CommonFeatureTrue"
	GovmomiFeature                     featuregate.Feature = "GovmomiFeature"
	SupervisorFeature                  featuregate.Feature = "SupervisorFeature"
	SupervisorFeatureEnabledOnV1Alpha2 featuregate.Feature = "SupervisorFeatureEnabledOnV1Alpha2"
	SupervisorFeatureEnabledOnV1Alpha5 featuregate.Feature = "SupervisorFeatureEnabledOnV1Alpha5"
	SupervisorFeatureEnabledOnV1Alpha6 featuregate.Feature = "SupervisorFeatureEnabledOnV1Alpha6"
)

var (
	commonTestGates = map[featuregate.Feature]featuregate.FeatureSpec{
		CommonFeature:     {Default: false, PreRelease: featuregate.Alpha},
		CommonFeatureTrue: {Default: true, PreRelease: featuregate.Beta},
	}

	govmomiTestGates = map[featuregate.Feature]featuregate.FeatureSpec{
		GovmomiFeature: {Default: false, PreRelease: featuregate.Alpha},
	}

	supervisorTestGates = map[featuregate.Feature]featuregate.FeatureSpec{
		SupervisorFeature: {Default: false, PreRelease: featuregate.Alpha},
	}

	supervisorVersionedTestGates = map[featuregate.Feature]featuregate.VersionedSpecs{
		SupervisorFeatureEnabledOnV1Alpha2: {
			{Version: toFeatureVersion(vmoprv1alpha2.GroupVersion.Version), Default: false, PreRelease: featuregate.Alpha},
		},
		SupervisorFeatureEnabledOnV1Alpha5: {
			{Version: toFeatureVersion(vmoprv1alpha5.GroupVersion.Version), Default: false, PreRelease: featuregate.Alpha},
		},
		SupervisorFeatureEnabledOnV1Alpha6: {
			{Version: toFeatureVersion(vmoprv1alpha6.GroupVersion.Version), Default: false, PreRelease: featuregate.Alpha},
		},
	}

	supportedVMOperatorAPIVersions = []string{vmoprv1alpha2.GroupVersion.Version, vmoprv1alpha5.GroupVersion.Version, vmoprv1alpha6.GroupVersion.Version}
)

func TestGetFlagDescription(t *testing.T) {
	g := NewWithT(t)

	description := getFlagDescription(commonTestGates, govmomiTestGates, supervisorTestGates, supervisorVersionedTestGates, supportedVMOperatorAPIVersions)

	g.Expect(description).To(Equal("" + "" +
		"A set of key=value pairs that describe feature gates for alpha/experimental features.\n" +
		"Common options are:\n" +
		"  AllAlpha=true|false (ALPHA - default=false)\n" +
		"  AllBeta=true|false (BETA - default=false)\n" +
		"  CommonFeature=true|false (ALPHA - default=false)\n" +
		"  CommonFeatureTrue=true|false (BETA - default=true)\n" +
		"Options for govmomi mode are:\n" +
		"  GovmomiFeature=true|false (ALPHA - default=false)\n" +
		"Options for supervisor mode when --vm-operator-api-version=v1alpha2 are:\n" +
		"  SupervisorFeature=true|false (ALPHA - default=false)\n" +
		"  SupervisorFeatureEnabledOnV1Alpha2=true|false (ALPHA - default=false)\n" +
		"Options for supervisor mode when --vm-operator-api-version=v1alpha5 are:\n" +
		"  SupervisorFeature=true|false (ALPHA - default=false)\n" +
		"  SupervisorFeatureEnabledOnV1Alpha2=true|false (ALPHA - default=false)\n" +
		"  SupervisorFeatureEnabledOnV1Alpha5=true|false (ALPHA - default=false)\n" +
		"Options for supervisor mode when --vm-operator-api-version=v1alpha6 are:\n" +
		"  SupervisorFeature=true|false (ALPHA - default=false)\n" +
		"  SupervisorFeatureEnabledOnV1Alpha2=true|false (ALPHA - default=false)\n" +
		"  SupervisorFeatureEnabledOnV1Alpha5=true|false (ALPHA - default=false)\n" +
		"  SupervisorFeatureEnabledOnV1Alpha6=true|false (ALPHA - default=false)\n"))
}

func TestGetGovmomiGates(t *testing.T) {
	g := NewWithT(t)

	allTestGates := featuregate.NewVersionedFeatureGate(toFeatureVersion(vmoprv1alpha6.GroupVersion.Version))
	utilruntime.Must(allTestGates.Add(commonTestGates))
	utilruntime.Must(allTestGates.Add(govmomiTestGates))
	utilruntime.Must(allTestGates.Add(supervisorTestGates))
	utilruntime.Must(allTestGates.AddVersioned(supervisorVersionedTestGates))

	allowedGates := getGovmomiGates(commonTestGates, govmomiTestGates)

	// default (no feature gates set)
	err := set(allTestGates, allowedGates, nil, "")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(GovmomiFeature)).To(BeFalse())

	// set a known feature gate
	err = set(allTestGates, allowedGates, nil, "GovmomiFeature=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(GovmomiFeature)).To(BeTrue())

	// set unknown feature gates
	err = set(allTestGates, allowedGates, nil, "SupervisorFeature=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: SupervisorFeature")})))

	err = set(allTestGates, allowedGates, nil, "SupervisorFeatureEnabledOnV1Alpha2=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: SupervisorFeatureEnabledOnV1Alpha2")})))

	err = set(allTestGates, allowedGates, nil, "Foo=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: Foo")})))
}

func TestGetSupervisorGatesV1Alpha2(t *testing.T) {
	g := NewWithT(t)

	allTestGates := featuregate.NewVersionedFeatureGate(toFeatureVersion(vmoprv1alpha5.GroupVersion.Version))
	utilruntime.Must(allTestGates.Add(commonTestGates))
	utilruntime.Must(allTestGates.Add(govmomiTestGates))
	utilruntime.Must(allTestGates.Add(supervisorTestGates))
	utilruntime.Must(allTestGates.AddVersioned(supervisorVersionedTestGates))

	allowedGates := getSupervisorGates(commonTestGates, supervisorTestGates, supervisorVersionedTestGates, vmoprv1alpha2.GroupVersion.Version)

	// default (no feature gates set)
	err := set(allTestGates, allowedGates, supervisorVersionedTestGates, "")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())

	// set a known feature gate (not versioned)
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeature=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())
	g.Expect(allTestGates.ResetFeatureValueToDefault(SupervisorFeature)).To(Succeed())

	// set a known versioned feature gate, supported in this version
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha2=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())
	g.Expect(allTestGates.ResetFeatureValueToDefault(SupervisorFeatureEnabledOnV1Alpha2)).To(Succeed())

	// set a known versioned feature(v1alpha5), NOT supported in this version, set to false
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha5=false")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())

	// set a known versioned feature(v1alpha5), NOT supported in this version, set to true
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha5=true")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("cannot set feature gate SupervisorFeatureEnabledOnV1Alpha5 to true, feature requires a newer vm-operator API version")})))

	// set a known versioned feature(v1alpha6), NOT supported in this version, set to false
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha6=false")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha6)).To(BeFalse())

	// set a known versioned feature(v1alpha6), NOT supported in this version, set to true
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha6=true")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("cannot set feature gate SupervisorFeatureEnabledOnV1Alpha6 to true, feature requires a newer vm-operator API version")})))

	// set unknown feature gates
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "GovmomiFeature=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: GovmomiFeature")})))

	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "Foo=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: Foo")})))
}

func TestGetSupervisorGatesV1Alpha5(t *testing.T) {
	g := NewWithT(t)

	allTestGates := featuregate.NewVersionedFeatureGate(toFeatureVersion(vmoprv1alpha5.GroupVersion.Version))
	utilruntime.Must(allTestGates.Add(commonTestGates))
	utilruntime.Must(allTestGates.Add(govmomiTestGates))
	utilruntime.Must(allTestGates.Add(supervisorTestGates))
	utilruntime.Must(allTestGates.AddVersioned(supervisorVersionedTestGates))

	allowedGates := getSupervisorGates(commonTestGates, supervisorTestGates, supervisorVersionedTestGates, vmoprv1alpha5.GroupVersion.Version)

	// default (no feature gates set)
	err := set(allTestGates, allowedGates, supervisorVersionedTestGates, "")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())

	// set a known feature gate (not versioned)
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeature=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())
	g.Expect(allTestGates.ResetFeatureValueToDefault(SupervisorFeature)).To(Succeed())

	// set a known versioned feature gate, supported in this version
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha2=true,SupervisorFeatureEnabledOnV1Alpha5=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeTrue())

	// set a known versioned feature(v1alpha6), NOT supported in this version, set to false
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha6=false")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha6)).To(BeFalse())

	// set a known versioned feature(v1alpha6), NOT supported in this version, set to true
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha6=true")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("cannot set feature gate SupervisorFeatureEnabledOnV1Alpha6 to true, feature requires a newer vm-operator API version")})))

	// set unknown feature gates
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "GovmomiFeature=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: GovmomiFeature")})))

	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "Foo=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: Foo")})))
}

func TestGetSupervisorGatesV1Alpha6(t *testing.T) {
	g := NewWithT(t)

	allTestGates := featuregate.NewVersionedFeatureGate(toFeatureVersion(vmoprv1alpha6.GroupVersion.Version))
	utilruntime.Must(allTestGates.Add(commonTestGates))
	utilruntime.Must(allTestGates.Add(govmomiTestGates))
	utilruntime.Must(allTestGates.Add(supervisorTestGates))
	utilruntime.Must(allTestGates.AddVersioned(supervisorVersionedTestGates))

	allowedGates := getSupervisorGates(commonTestGates, supervisorTestGates, supervisorVersionedTestGates, vmoprv1alpha6.GroupVersion.Version)

	// default (no feature gates set)
	err := set(allTestGates, allowedGates, supervisorVersionedTestGates, "")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha6)).To(BeFalse())

	// set a known feature gate (not versioned)
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeature=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha6)).To(BeFalse())
	g.Expect(allTestGates.ResetFeatureValueToDefault(SupervisorFeature)).To(Succeed())

	// set a known versioned feature gate, supported in this version
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "SupervisorFeatureEnabledOnV1Alpha2=true,SupervisorFeatureEnabledOnV1Alpha5=true,SupervisorFeatureEnabledOnV1Alpha6=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allTestGates.Enabled(CommonFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(CommonFeatureTrue)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeature)).To(BeFalse())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha2)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha5)).To(BeTrue())
	g.Expect(allTestGates.Enabled(SupervisorFeatureEnabledOnV1Alpha6)).To(BeTrue())

	// set unknown feature gates
	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "GovmomiFeature=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: GovmomiFeature")})))

	err = set(allTestGates, allowedGates, supervisorVersionedTestGates, "Foo=false")
	g.Expect(err).To(Equal(kerrors.NewAggregate([]error{fmt.Errorf("unrecognized feature gate: Foo")})))
}

func TestVLANSubinterfaceFeatureGate(t *testing.T) {
	g := NewWithT(t)

	// Test with v1alpha5 - VLANSubinterface should not be allowed to be set to true
	allowedGatesV1Alpha5 := getSupervisorGates(commonGates, supervisorGates, supervisorVersionedGates, vmoprv1alpha5.GroupVersion.Version)
	allGatesV1Alpha5 := featuregate.NewVersionedFeatureGate(toFeatureVersion(vmoprv1alpha5.GroupVersion.Version))
	utilruntime.Must(allGatesV1Alpha5.Add(commonGates))
	utilruntime.Must(allGatesV1Alpha5.Add(supervisorGates))
	utilruntime.Must(allGatesV1Alpha5.AddVersioned(supervisorVersionedGates))

	err := set(allGatesV1Alpha5, allowedGatesV1Alpha5, supervisorVersionedGates, "VLANSubinterface=true")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot set feature gate VLANSubinterface to true, feature requires a newer vm-operator API version"))

	// Test with v1alpha6 - VLANSubinterface should be allowed to be set to true
	allowedGatesV1Alpha6 := getSupervisorGates(commonGates, supervisorGates, supervisorVersionedGates, vmoprv1alpha6.GroupVersion.Version)
	allGatesV1Alpha6 := featuregate.NewVersionedFeatureGate(toFeatureVersion(vmoprv1alpha6.GroupVersion.Version))
	utilruntime.Must(allGatesV1Alpha6.Add(commonGates))
	utilruntime.Must(allGatesV1Alpha6.Add(supervisorGates))
	utilruntime.Must(allGatesV1Alpha6.AddVersioned(supervisorVersionedGates))

	err = set(allGatesV1Alpha6, allowedGatesV1Alpha6, supervisorVersionedGates, "VLANSubinterface=true")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(allGatesV1Alpha6.Enabled(VLANSubinterface)).To(BeTrue())
}
