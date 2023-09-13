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

// Package cmd executes flavorgen commands.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/util"
)

const flavorFlag = "flavor"
const outputDirFlag = "output-dir"

var (
	flavorMappings = map[string]string{
		flavors.VIP:                  "cluster-template.yaml",
		flavors.ExternalLoadBalancer: "cluster-template-external-loadbalancer.yaml",
		flavors.ClusterClass:         "clusterclass-template.yaml",
		flavors.ClusterTopology:      "cluster-template-topology.yaml",
		flavors.Ignition:             "cluster-template-ignition.yaml",
		flavors.NodeIPAM:             "cluster-template-node-ipam.yaml",
	}

	allFlavors = []string{
		flavors.VIP,
		flavors.ExternalLoadBalancer,
		flavors.ClusterClass,
		flavors.Ignition,
		flavors.NodeIPAM,
		flavors.ClusterTopology,
	}
)

func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "flavorgen",
		Short: "flavorgen generates clusterctl templates for Cluster API Provider vSphere",
		RunE: func(command *cobra.Command, args []string) error {
			return RunRoot(command)
		},
		SilenceUsage: true,
	}
	rootCmd.Flags().StringP(flavorFlag, "f", "", "Name of flavor to compile")
	rootCmd.Flags().StringP(outputDirFlag, "o", "", "Directory to store the generated flavor templates.\nBy default the current directory is used.\nUse '-' to output the result to stdout.")

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
	outputDir, err := command.Flags().GetString(outputDirFlag)
	if err != nil {
		return errors.Wrapf(err, "error accessing flag %s for command %s", outputDirFlag, command.Name())
	}
	var outputFlavors []string
	if flavor != "" {
		outputFlavors = append(outputFlavors, flavor)
	} else {
		outputFlavors = allFlavors
	}
	generateMultiFlavors := len(outputFlavors) > 1
	for _, f := range outputFlavors {
		manifest, err := generateSingle(f)
		if err != nil {
			return err
		}

		yamlFileName, ok := flavorMappings[f]
		if !ok {
			return fmt.Errorf("file mapping for flavor %q is missng in flavorMappings", f)
		}

		if outputDir == "-" {
			if generateMultiFlavors {
				// use the yaml filename as a section delimiter
				fmt.Printf("### %s\n", yamlFileName)
			}
			fmt.Print(manifest)
			continue
		}

		yamlPath := filepath.Join(outputDir, yamlFileName)
		err = os.WriteFile(yamlPath, []byte(manifest), 0600)
		if err != nil {
			return errors.Wrapf(err, "failed to save manifest content to file for flavor %s", f)
		}
	}

	return nil
}

func generateSingle(flavor string) (string, error) {
	replacements := append([]util.Replacement{}, util.DefaultReplacements...)

	var objs []runtime.Object
	switch flavor {
	case flavors.VIP:
		var err error
		objs, err = flavors.MultiNodeTemplateWithKubeVIP()
		if err != nil {
			return "", err
		}
	case flavors.ExternalLoadBalancer:
		var err error
		objs, err = flavors.MultiNodeTemplateWithExternalLoadBalancer()
		if err != nil {
			return "", err
		}
	case flavors.ClusterClass:
		objs = flavors.ClusterClassTemplateWithKubeVIP()
	case flavors.ClusterTopology:
		var err error
		objs, err = flavors.ClusterTopologyTemplateKubeVIP()
		if err != nil {
			return "", err
		}
		replacements = append(replacements, util.Replacement{
			Kind:      "Cluster",
			Name:      "${CLUSTER_NAME}",
			Value:     env.ControlPlaneMachineCountVar,
			FieldPath: []string{"spec", "topology", "controlPlane", "replicas"},
		})
	case flavors.Ignition:
		var err error
		objs, err = flavors.MultiNodeTemplateWithKubeVIPIgnition()
		if err != nil {
			return "", err
		}
	case flavors.NodeIPAM:
		var err error
		objs, err = flavors.MultiNodeTemplateWithKubeVIPNodeIPAM()
		if err != nil {
			return "", err
		}
	default:
		return "", errors.Errorf("invalid flavor")
	}

	return util.GenerateManifestYaml(objs, replacements), nil
}
