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

// Package util contains common tools for flavorgen.
package util

import (
	"reflect"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-provider-vsphere/packaging/flavorgen/flavors/env"
)

type Replacement struct {
	Kind      string
	Name      string
	Value     interface{}
	FieldPath []string
}

var (
	DefaultReplacements = []Replacement{
		{
			Kind:      "KubeadmControlPlane",
			Name:      "${CLUSTER_NAME}",
			Value:     env.ControlPlaneMachineCountVar,
			FieldPath: []string{"spec", "replicas"},
		},
		{
			Kind:      "MachineDeployment",
			Name:      "${CLUSTER_NAME}-md-0",
			Value:     env.WorkerMachineCountVar,
			FieldPath: []string{"spec", "replicas"},
		},
		{
			Kind:      "MachineDeployment",
			Name:      "${CLUSTER_NAME}-md-0",
			Value:     map[string]interface{}{},
			FieldPath: []string{"spec", "selector", "matchLabels"},
		},
		{
			Kind:      "VSphereClusterTemplate",
			Name:      "${CLUSTER_CLASS_NAME}",
			Value:     map[string]interface{}{},
			FieldPath: []string{"spec", "template", "spec"},
		},
	}

	stringVars = []string{
		regexVar(env.ClusterNameVar),
		regexVar(env.ClusterClassNameVar),
		regexVar(env.ClusterNameVar + env.MachineDeploymentNameSuffix),
		regexVar(env.NamespaceVar),
		regexVar(env.KubernetesVersionVar),
		regexVar(env.VSphereFolderVar),
		regexVar(env.VSphereResourcePoolVar),
		regexVar(env.VSphereSSHAuthorizedKeysVar),
		regexVar(env.VSphereDataCenterVar),
		regexVar(env.VSphereDatastoreVar),
		regexVar(env.VSphereNetworkVar),
		regexVar(env.VSphereServerVar),
		regexVar(env.VSphereTemplateVar),
		regexVar(env.VSphereStoragePolicyVar),
		// TODO: Why was thumbprint not here?
		regexVar(env.VSphereThumbprint),
	}
)

func regexVar(str string) string {
	return "((?m:\\" + str + "$))"
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0 || v.IsZero()
	case reflect.Struct:
		return v.IsZero() || v.IsNil() || v.IsZero()
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0 || v.IsNil()
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil() || v.IsZero()
	}
	return false
}

func deleteZeroValues(o map[string]interface{}) map[string]interface{} {
	for k, v := range o {
		val := reflect.ValueOf(v)
		if v == nil || isZeroValue(val) || !val.IsValid() {
			delete(o, k)
			continue
		}
		if val.Kind() == reflect.Map {
			newMap := v.(map[string]interface{})
			newMap = deleteZeroValues(newMap)
			if isZeroValue(reflect.ValueOf(newMap)) {
				delete(o, k)
			}
			continue
		}
	}
	return o
}

func GenerateObjectYAML(obj runtime.Object, replacements []Replacement) string {
	bytes, err := yaml.Marshal(obj)
	if err != nil {
		panic(err)
	}
	json, err := yaml.YAMLToJSONStrict(bytes)
	if err != nil {
		panic(err)
	}

	data := unstructured.Unstructured{}
	if err := data.UnmarshalJSON(json); err != nil {
		panic(err)
	}
	data.Object = deleteZeroValues(data.Object)

	for _, v := range replacements {
		v := v
		if v.Name == data.GetName() && v.Kind == data.GetKind() {
			if err := unstructured.SetNestedField(data.Object, v.Value, v.FieldPath...); err != nil {
				panic(err)
			}
		}
	}
	// In the future, if we need to replace nested slice for some other reason,
	// we could consider creating another utility for it and move this out.
	if data.GetKind() == "Cluster" {
		path := []string{"spec", "topology", "workers", "machineDeployments"}
		slice, found, err := unstructured.NestedSlice(data.Object, path...)
		if found && err == nil {
			slice[0].(map[string]interface{})["replicas"] = env.WorkerMachineCountVar
			_ = unstructured.SetNestedSlice(data.Object, slice, path...)
		}
	}
	bytes, err = yaml.Marshal(data.Object)
	if err != nil {
		panic(err)
	}

	str := string(bytes)

	for _, s := range stringVars {
		s := s
		regex := regexp.MustCompile(s)
		if err != nil {
			panic(err)
		}
		str = regex.ReplaceAllString(str, "'$1'")
	}

	return str
}

func GenerateManifestYaml(objs []runtime.Object, replacements []Replacement) string {
	var sb strings.Builder

	for _, o := range objs {
		sb.WriteString("---\n")
		sb.WriteString(GenerateObjectYAML(o, replacements))
	}

	return sb.String()
}

func TypeToKind(i interface{}) string {
	return reflect.ValueOf(i).Elem().Type().Name()
}
