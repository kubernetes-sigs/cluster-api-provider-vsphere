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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "sigs.k8s.io/cluster-api/util/yaml"

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
		{
			Kind:      "VSphereCluster",
			Name:      "${CLUSTER_NAME}",
			Value:     env.ControlPlaneEndpointPortVar,
			FieldPath: []string{"spec", "controlPlaneEndpoint", "port"},
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
		regexVar(env.VSphereThumbprint),
	}

	stringVarsDouble = []string{
		regexVar(env.VSphereUsername),
		regexVar(env.VSpherePassword),
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
	data, err := toUnstructured(obj, obj.GetObjectKind().GroupVersionKind())
	if err != nil {
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

	bytes, err := utilyaml.FromUnstructured([]unstructured.Unstructured{*data})
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
	for _, s := range stringVarsDouble {
		s := s
		regex := regexp.MustCompile(s)
		if err != nil {
			panic(err)
		}
		str = regex.ReplaceAllString(str, "\"$1\"")
	}

	return str
}

func GenerateManifestYaml(objs []runtime.Object, replacements []Replacement) string {
	bytes := [][]byte{}
	for _, o := range objs {
		bytes = append(bytes, []byte(GenerateObjectYAML(o, replacements)))
	}

	return string(utilyaml.JoinYaml(bytes...))
}

func TypeToKind(i interface{}) string {
	return reflect.ValueOf(i).Elem().Type().Name()
}

// toUnstructured converts an object to Unstructured.
// We have to pass in a gvk as we can't rely on GVK being set in a runtime.Object.
func toUnstructured(obj runtime.Object, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	// If the incoming object is already unstructured, perform a deep copy first
	// otherwise DefaultUnstructuredConverter ends up returning the inner map without
	// making a copy.
	if _, ok := obj.(runtime.Unstructured); ok {
		obj = obj.DeepCopyObject()
	}
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: rawMap}
	u.SetGroupVersionKind(gvk)

	return u, nil
}
