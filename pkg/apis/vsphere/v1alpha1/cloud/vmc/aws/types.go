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

// Package aws contains API for VMC on AWS.
package aws

// VmwareCloudSpec describes the supported cloud providers
type VmwareCloudSpec struct {
	// AwsProvider specifies the information needed to run VMC on AWS
	AwsProvider *ProviderSpec `json:"awsProviderSpec"`
}

// ProviderSpec specifies the information needed to run VMC on AWS
type ProviderSpec struct {
	// VpcID is the id of the VPC used to create loadBalancers
	VpcID string `json:"vpcID"`

	// Subnets is the list of subnets where
	Subnets []string `json:"subnets"`

	// Region is the region used for the loadbalancers
	Region string `json:"region"`
}
