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

package builder

import (
	"flag"
	"os"
	"strings"
)

type TestFlags struct {
	// rootDir is the root directory of the checked-out project and is set with
	// the -root-dir flag.
	// Defaults to ../../
	RootDir string

	// integrationTestsEnabled is set to true with the -enable-integration-tests
	// flag.
	// Defaults to false.
	IntegrationTestsEnabled bool
}

var flags TestFlags

func init() {
	flags = TestFlags{}
	// Create a special flagset used to parse the -enable-integration-tests
	// and -enable-unit-tests flags. A special flagset is used so as not to
	// interfere with whatever Ginkgo or Kubernetes might be doing with the
	// default flagset.
	//
	// Please note that in order for this to work, we must copy the os.Args
	// slice into a new slice, removing any flags except those about which
	// we're concerned and possibly values immediately succeeding those flags,
	// provided the values are not prefixed with a "-" character.
	cmdLine := flag.NewFlagSet("test", flag.PanicOnError)
	var args []string
	for i := 0; i < len(os.Args); {
		if strings.HasPrefix(os.Args[i], "-enable-integration-tests") || strings.HasPrefix(os.Args[i], "-enable-unit-tests") {
			args = append(args, os.Args[i])
		}
		i++
	}
	cmdLine.BoolVar(&flags.IntegrationTestsEnabled, "enable-integration-tests", false, "Enables integration tests")
	_ = cmdLine.Parse(args)

	// We still need to add the flags to the default flagset, because otherwise
	// Ginkgo will complain that the flags are not recognized.
	flag.Bool("enable-integration-tests", false, "Enables integration tests")
	flag.Bool("enable-unit-tests", true, "Enables unit tests")
	flag.StringVar(&flags.RootDir, "root-dir", "../..", "Root project directory")
}

func GetTestFlags() TestFlags {
	return flags
}
