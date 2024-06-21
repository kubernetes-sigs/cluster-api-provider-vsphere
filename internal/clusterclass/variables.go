/*
Copyright 2024 The Kubernetes Authors.

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

// Package clusterclass provides the shared functions for creating clusterclasses.
package clusterclass

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// GetClusterClassVariables provides the variables for the clusterclass.
// In govmomi mode it has additional variables.
func GetClusterClassVariables(govmomiMode bool) []clusterv1.ClusterClassVariable {
	variables := []clusterv1.ClusterClassVariable{
		{
			Name:     "sshKey",
			Required: false,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Description: "Public key to SSH onto the cluster nodes.",
					Type:        "string",
				},
			},
		},
		{
			Name:     "controlPlaneIpAddr",
			Required: true,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Type:        "string",
					Description: "Floating VIP for the control plane.",
				},
			},
		},
		{
			Name:     "controlPlanePort",
			Required: true,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Type:        "integer",
					Description: "Port for the control plane endpoint.",
				},
			},
		},
		{
			Name:     "kubeVipPodManifest",
			Required: true,
			Schema: clusterv1.VariableSchema{
				OpenAPIV3Schema: clusterv1.JSONSchemaProps{
					Type:        "string",
					Description: "kube-vip manifest for the control plane.",
				},
			},
		},
	}

	if govmomiMode {
		varForNoneSupervisorMode := []clusterv1.ClusterClassVariable{
			{
				Name:     "infraServer",
				Required: true,
				Schema: clusterv1.VariableSchema{
					OpenAPIV3Schema: clusterv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]clusterv1.JSONSchemaProps{
							"url":        {Type: "string"},
							"thumbprint": {Type: "string"},
						},
					},
				},
			},
			{
				Name:     "credsSecretName",
				Required: true,
				Schema: clusterv1.VariableSchema{
					OpenAPIV3Schema: clusterv1.JSONSchemaProps{
						Type:        "string",
						Description: "Secret containing the credentials for the infra cluster.",
					},
				},
			},
		}

		variables = append(variables, varForNoneSupervisorMode...)
	}

	return variables
}
