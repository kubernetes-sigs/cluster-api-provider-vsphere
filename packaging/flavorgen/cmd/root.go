/*
Copyright 2020 The Kubernetes Authors.

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

package cmd

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
)

const flavorFlag = "flavor"

func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "flavorgen",
		Short: "flavorgen generates clusterctl templates for Cluster API Provider vSphere",
		RunE: func(command *cobra.Command, args []string) error {
			return RunRoot(command)
		},
	}
	rootCmd.Flags().StringP(flavorFlag, "f", "", "Name of flavor to compile")
	return rootCmd
}

func Execute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func RunRoot(command *cobra.Command) error {
	flavor, err := command.Flags().GetString(flavorFlag)
	if err != nil {
		return errors.Wrapf(err, "error accessing flag %s for command %s", flavorFlag, command.Name())
	}
	switch flavor {
	case "vip":
		util.PrintObjects(flavors.MultiNodeTemplateWithKubeVIP())
	case "external-loadbalancer":
		util.PrintObjects(flavors.MultiNodeTemplateWithExternalLoadBalancer())
	default:
		return errors.Errorf("invalid flavor")
	}
	return nil
}
