/*
Copyright 2019 The Kubernetes Authors.

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

package app

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/cmd/manifests/pkg/encoding/slim"
	"sigs.k8s.io/cluster-api-provider-vsphere/cmd/manifests/pkg/kustomize"
)

// StringSliceFlag may be used with flag.Var to register a flag that may be
// specified multiple times on the command line and return a slice of the args.
type StringSliceFlag []string

// String returns the list as a CSV string.
func (s *StringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

// Get returns the underlying string slice.
func (s *StringSliceFlag) Get() []string {
	return *s
}

// Set is called once, in command line order, for each flag present.
func (s *StringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

var (
	configDirs        StringSliceFlag
	kubernetesVersion = flag.String(
		"kubernetes-version",
		"v1.13.6",
		"The version of Kubernetes to deploy")
	clusterName = flag.String(
		"cluster-name",
		"management-cluster",
		"The name of the cluster")
	podCIDR = flag.String(
		"pod-cidr",
		"100.96.0.0/11",
		"The CIDR for the cluster's pod network.")
	controlPlaneEndpoint = flag.String(
		"control-plane-endpoint",
		"",
		"The control plane endpoint to use.")
	numControlPlaneMachines = flag.Int(
		"num-control-plane-machines",
		1,
		"The number of machines that belong to the control plane.")
	numWorkerMachines = flag.Int(
		"num-worker-machines",
		2,
		"The number of machines that are strictly worker nodes.")
	serviceCIDR = flag.String(
		"service-cidr",
		"100.64.0.0/13",
		"The CIDR for the cluster's service network.")
	serviceDomain = flag.String(
		"service-domain",
		"cluster.local",
		"The domain name for the cluster's service network.")
	clusterOutPath = flag.String(
		"cluster-out",
		"cluster.yaml",
		"The path to write the generated cluster manifest")
	machinesOutPath = flag.String(
		"machines-out",
		"machines.yaml",
		"The path to write the generated machines manifest")
	machineSetOutPath = flag.String(
		"machine-set-out",
		"machineset.yaml",
		"The path to write the generated machineset manifest")
	providerComponentsOutPath = flag.String(
		"provider-components-out",
		"provider-components.yaml",
		"The path to write the generated provider components manifest")
	addonsInPath = flag.String(
		"addons-in",
		"",
		"The addons manifest template to interpolate. If omitted then a default manifest with Calico is used")
	addonsOutPath = flag.String(
		"addons-out",
		"addons.yaml",
		"The path to write the interpolated addons manifest")
)

func init() {
	flag.Var(&configDirs,
		"config-dir",
		"A directory containing Kustomization resources. May be specified more than once.")
}

// Provider describes a type that can return a Cluster & Machine provider spec.
type Provider interface {
	GetClusterProviderSpec() (runtime.Object, error)
	GetMachineProviderSpec() (runtime.Object, error)
}

// Run is the entry point for the application.
func Run(p Provider) error {
	if !flag.Parsed() {
		flag.Parse()
	}

	// Resolve all of the config dirs to absolute paths.
	for i := range configDirs {
		absPath, err := filepath.Abs(configDirs[i])
		if err != nil {
			return err
		}
		configDirs[i] = absPath
	}

	// Create the tempalte data from the flags.
	templateData := createTemplateData(flag.CommandLine)

	if err := generateClusterManifest(p); err != nil {
		return err
	}
	if err := generateMachinesManifest(p); err != nil {
		return err
	}
	if err := generateMachineSetManifest(p); err != nil {
		return err
	}
	if err := generateProviderComponentsManifest(p, templateData); err != nil {
		return err
	}
	if err := generateAddonsManifest(templateData); err != nil {
		return err
	}

	return nil
}

func createTemplateData(flagSet *flag.FlagSet) map[string]interface{} {
	templateData := map[string]interface{}{}
	flagSet.VisitAll(func(f *flag.Flag) {
		getter, ok := f.Value.(flag.Getter)
		if !ok {
			return
		}

		// Get the camel-cased version of the flag name.
		flagNameParts := strings.Split(f.Name, "-")
		flagNameData := &bytes.Buffer{}
		for i := range flagNameParts {
			flagNameData.WriteString(strings.Title(flagNameParts[i]))
		}
		flagName := flagNameData.String()

		// Add the flag's value to the template data.
		templateData[flagName] = getter.Get()
	})
	return templateData
}

func generateProviderComponentsManifest(p Provider, templateData map[string]interface{}) error {
	fout, err := os.Create(*providerComponentsOutPath)
	if err != nil {
		return err
	}
	defer fout.Close()
	for i, configDirPath := range configDirs {
		buildOptions := &kustomize.BuildOptions{
			Out:               fout,
			KustomizationPath: configDirPath,
			TemplateData:      templateData,
		}
		if err := kustomize.RunBuild(buildOptions); err != nil {
			return errors.Wrap(err, "failed to run kustomize")
		}
		if i < len(configDirs)-1 {
			if _, err := fmt.Fprintf(fout, "---\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateAddonsManifest(templateData map[string]interface{}) error {
	var tpl *template.Template
	if *addonsInPath == "" {
		var err error
		if tpl, err = template.New("t").Parse(addonsManifestFormat); err != nil {
			return err
		}
	} else {
		var err error
		if tpl, err = template.ParseFiles(*addonsInPath); err != nil {
			return err
		}
	}
	fout, err := os.Create(*addonsOutPath)
	if err != nil {
		return err
	}
	defer fout.Close()
	return tpl.Execute(fout, templateData)
}

func generateClusterManifest(p Provider) error {
	providerSpec, err := p.GetClusterProviderSpec()
	if err != nil {
		return err
	}

	encodedProviderSpec, err := slim.EncodeAsRawExtension(providerSpec)
	if err != nil {
		return err
	}

	obj := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.SchemeGroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: *clusterName,
		},
		Spec: clusterv1.ClusterSpec{
			ProviderSpec: clusterv1.ProviderSpec{
				Value: encodedProviderSpec,
			},
			ClusterNetwork: clusterv1.ClusterNetworkingConfig{
				Pods: clusterv1.NetworkRanges{
					CIDRBlocks: []string{*podCIDR},
				},
				ServiceDomain: *serviceDomain,
				Services: clusterv1.NetworkRanges{
					CIDRBlocks: []string{*serviceCIDR},
				},
			},
		},
	}

	return writeObjToFile(obj, *clusterOutPath)
}

func generateMachinesManifest(p Provider) error {
	providerSpec, err := p.GetMachineProviderSpec()
	if err != nil {
		return err
	}

	encodedProviderSpec, err := slim.EncodeAsRawExtension(providerSpec)
	if err != nil {
		return err
	}

	obj := &clusterv1.MachineList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.SchemeGroupVersion.String(),
			Kind:       "MachineList",
		},
		Items: make([]clusterv1.Machine, *numControlPlaneMachines),
	}

	for i := range obj.Items {
		obj.Items[i] = clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-controlplane-%d", *clusterName, *numControlPlaneMachines),
				Labels: map[string]string{
					clusterv1.MachineClusterLabelName: *clusterName,
				},
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: clusterv1.SchemeGroupVersion.String(),
				Kind:       "Machine",
			},
			Spec: clusterv1.MachineSpec{
				ProviderSpec: clusterv1.ProviderSpec{
					Value: encodedProviderSpec,
				},
				Versions: clusterv1.MachineVersionInfo{
					Kubelet:      *kubernetesVersion,
					ControlPlane: *kubernetesVersion,
				},
			},
		}
	}

	return writeObjToFile(obj, *machinesOutPath)
}

func generateMachineSetManifest(p Provider) error {
	providerSpec, err := p.GetMachineProviderSpec()
	if err != nil {
		return err
	}

	encodedProviderSpec, err := slim.EncodeAsRawExtension(providerSpec)
	if err != nil {
		return err
	}

	objName := fmt.Sprintf("%s-machineset-1", *clusterName)
	objAndSelectorAndTemplateLabels := map[string]string{
		"machineset-name":                 objName,
		clusterv1.MachineClusterLabelName: *clusterName,
	}
	obj := &clusterv1.MachineSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.SchemeGroupVersion.String(),
			Kind:       "MachineSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   objName,
			Labels: objAndSelectorAndTemplateLabels,
		},
		Spec: clusterv1.MachineSetSpec{
			Replicas: addrInt32(numWorkerMachines),
			Selector: metav1.LabelSelector{
				MatchLabels: objAndSelectorAndTemplateLabels,
			},
			Template: clusterv1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: objAndSelectorAndTemplateLabels,
				},
				Spec: clusterv1.MachineSpec{
					ProviderSpec: clusterv1.ProviderSpec{
						Value: encodedProviderSpec,
					},
					Versions: clusterv1.MachineVersionInfo{
						Kubelet: *kubernetesVersion,
					},
				},
			},
		},
	}

	return writeObjToFile(obj, *machineSetOutPath)
}

func addrInt32(v *int) *int32 {
	i := int32(*v)
	return &i
}

func writeObjToFile(obj runtime.Object, filePath string) error {
	objYAML, err := slim.MarshalYAML(obj)
	if err != nil {
		return err
	}

	fout, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer fout.Close()

	if _, err := io.Copy(fout, bytes.NewReader(objYAML)); err != nil {
		return err
	}

	return nil
}
