/*
Copyright 2021 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	vmoprv1alpha2 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/component-base/featuregate"
)

var (
	allGates featuregate.MutableVersionedFeatureGate

	// Gates is a shared global FeatureGate.
	Gates featuregate.FeatureGate
)

func init() {
	// Add all gates to avoid issue in test code that assumes gates are properly set up.
	// Note: When the controller starts, only a subset of those gates will be allowed depending on mode / vm-operator version.
	allGates = featuregate.NewVersionedFeatureGate(toFeatureVersion(vmoprv1alpha5.GroupVersion.Version))
	utilruntime.Must(allGates.Add(commonGates))
	utilruntime.Must(allGates.Add(govmomiGates))
	utilruntime.Must(allGates.Add(supervisorGates))
	utilruntime.Must(allGates.AddVersioned(supervisorVersionedGates))
	Gates = allGates
}

// AddFlag adds a flag for setting global feature gates to the specified FlagSet.
func AddFlag(fs *pflag.FlagSet, p *string, supportedVersions []string) {
	fs.StringVar(p, "feature-gates", "", getFlagDescription(commonGates, govmomiGates, supervisorGates, supervisorVersionedGates, supportedVersions))
}

// SetGovmomiGates set Gates for govmomi mode.
func SetGovmomiGates(p string) error {
	allowedGates := getGovmomiGates(commonGates, govmomiGates)
	return set(allGates, allowedGates, nil, p)
}

// SetSupervisorGates sets Gates for supervisor mode / for a specific vm-operator API version.
func SetSupervisorGates(vmOperatorAPIVersion string, p string) error {
	allowedGates := getSupervisorGates(commonGates, supervisorGates, supervisorVersionedGates, vmOperatorAPIVersion)
	return set(allGates, allowedGates, supervisorVersionedGates, p)
}

// getFlagDescription return description for the feature-gates flags that shows flags available in all the supported permutations of mode and vm-operator API version.
func getFlagDescription(commonGates, govmomiGates, supervisorGates map[featuregate.Feature]featuregate.FeatureSpec, supervisorVersionedGates map[featuregate.Feature]featuregate.VersionedSpecs, supportedVersions []string) string {
	commonMutableGates := featuregate.NewFeatureGate()
	utilruntime.Must(commonMutableGates.Add(commonGates))

	govmomiMutableGates := featuregate.NewFeatureGate()
	utilruntime.Must(govmomiMutableGates.Add(govmomiGates))

	// Generate flag description
	fix := func(know []string, dropAll bool) []string {
		r := []string{}
		for _, k := range know {
			if dropAll && (strings.HasPrefix(k, "AllAlpha=") || strings.HasPrefix(k, "AllBeta=")) {
				continue
			}
			r = append(r, fmt.Sprintf("  %s", k))
		}
		sort.Strings(r)
		return r
	}

	description := "A set of key=value pairs that describe feature gates for alpha/experimental features.\n"
	description += fmt.Sprintf("Common options are:\n%s\n", strings.Join(fix(commonMutableGates.KnownFeatures(), false), "\n"))
	description += fmt.Sprintf("Options for govmomi mode are:\n%s\n", strings.Join(fix(govmomiMutableGates.KnownFeatures(), true), "\n"))
	for _, v := range supportedVersions {
		supervisorMutableGates := featuregate.NewVersionedFeatureGate(toFeatureVersion(v))
		utilruntime.Must(supervisorMutableGates.Add(supervisorGates))
		utilruntime.Must(supervisorMutableGates.AddVersioned(supervisorVersionedGates))
		description += fmt.Sprintf("Options for supervisor mode when --vm-operator-api-version=%s are:\n%s\n", v, strings.Join(fix(supervisorMutableGates.KnownFeatures(), true), "\n"))
	}
	return description
}

// getGovmomiGates gets a featuregate.FeatureGate instance for govmomi mode.
func getGovmomiGates(commonGates, govmomiGates map[featuregate.Feature]featuregate.FeatureSpec) featuregate.MutableFeatureGate {
	mutableGates := featuregate.NewFeatureGate()
	utilruntime.Must(mutableGates.Add(commonGates))
	utilruntime.Must(mutableGates.Add(govmomiGates))
	mutableGates.Close()
	return mutableGates
}

// getSupervisorGates gets a featuregate.FeatureGate instance for supervisor mode / for a specific vm-operator API version.
func getSupervisorGates(commonGates, supervisorGates map[featuregate.Feature]featuregate.FeatureSpec, supervisorVersionedGates map[featuregate.Feature]featuregate.VersionedSpecs, vmOperatorAPIVersion string) featuregate.MutableFeatureGate {
	mutableGates := featuregate.NewVersionedFeatureGate(toFeatureVersion(vmOperatorAPIVersion))
	utilruntime.Must(mutableGates.Add(commonGates))
	utilruntime.Must(mutableGates.Add(supervisorGates))
	utilruntime.Must(mutableGates.AddVersioned(supervisorVersionedGates))
	mutableGates.Close()
	return mutableGates
}

func set(allGates, allowedGates featuregate.MutableFeatureGate, versionedGates map[featuregate.Feature]featuregate.VersionedSpecs, value string) error {
	// Kubernetes featuregate.MutableVersionedFeatureGate does not allow setting flags that are supported only in future version.
	// In CAPV we want to tolerate setting flags that are supported only in future version only if set to false.
	// The following code drops unknown flags set to false.
	versioned := sets.New[string]()
	for gate := range versionedGates {
		versioned.Insert(string(gate))
	}

	known := sets.New[string]()
	for _, gate := range allowedGates.KnownFeatures() {
		known.Insert(gate[0:strings.Index(gate, "=")])
	}

	var values []string
	for _, s := range strings.Split(value, ",") {
		if s == "" {
			continue
		}

		// Gets the name of the feature flag.
		arr := strings.SplitN(s, "=", 2)
		k := strings.TrimSpace(arr[0])
		if len(arr) != 2 {
			values = append(values, s)
			continue
		}

		// If it is not versioned, keep it in the list of gates.
		if !versioned.Has(k) {
			values = append(values, s)
			continue
		}

		// If it is versioned, drop it from the list of flags only if
		// it is not a know gate for the current version (it is supported in future versions only)
		// and the value is false.
		v := strings.TrimSpace(arr[1])
		boolValue, err := strconv.ParseBool(v)
		if err != nil {
			values = append(values, s)
			continue
		}
		if !boolValue && !known.Has(k) {
			continue
		}
		values = append(values, s)
	}
	cleanedValue := strings.Join(values, ",")

	// First set and validate feature gate value on the allowedGates.
	allowedGates = allowedGates.DeepCopy()
	err := allowedGates.Set(cleanedValue)
	if err != nil {
		if aggregateErr, ok := errors.AsType[kerrors.Aggregate](err); ok && aggregateErr != nil {
			cleanedErrors := make([]error, 0, len(aggregateErr.Errors()))
			for _, err := range aggregateErr.Errors() {
				// Surface all the errors except for the error that is generated when the user sets a featureFlag that is supported only in future version
				if !strings.Contains(err.Error(), "feature is PreAlpha at emulated version") {
					cleanedErrors = append(cleanedErrors, err)
					continue
				}

				// Kubernetes featuregate.MutableVersionedFeatureGate do not allow setting flags that are supported only in future version entirely.
				// In CAPV we want to tolerate setting flags that are supported only in future version only if set to false;
				// as a consequence we ignore messages that are generated in this case.
				// NOTE: this should not happen since we are filtering on Set.
				if strings.Contains(err.Error(), " to false,") {
					continue
				}

				// If instead the user sets a featureFlag that is supported only in future version to true,
				// change the error message so it is easier to understand for CAPV users.
				i := strings.Index(err.Error(), " to true,")
				cleanedErrors = append(cleanedErrors, fmt.Errorf("%s to true, feature requires a newer vm-operator API version", err.Error()[0:i]))
			}
			if len(cleanedErrors) > 0 {
				return kerrors.NewAggregate(cleanedErrors)
			}
		} else {
			return err
		}
	}

	// If there are no error, set values on allGates
	// Note: at this point there should not be errors.
	return allGates.Set(cleanedValue)
}

// toFeatureVersion transforms a vm-operator API version in a version.Version
// that can be used for versioned feature gates.
// Note: The value of the returned version.Version (major.minor) does not have any specific
// meaning, it is only intended to express v1alpha2 < v1alpha5 etc.
func toFeatureVersion(v string) *version.Version {
	versions := map[string]*version.Version{
		vmoprv1alpha2.GroupVersion.Version: version.MustParse("0.2"),
		vmoprv1alpha5.GroupVersion.Version: version.MustParse("0.5"),
	}
	featureVersion, ok := versions[v]
	if !ok {
		panic(fmt.Errorf("unknown vm-operator version: %s", v))
	}
	return featureVersion
}
