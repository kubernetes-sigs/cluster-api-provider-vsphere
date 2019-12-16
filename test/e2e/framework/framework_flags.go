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

package framework

import (
	"flag"
	"os"
)

const (
	// DefaultManagementClusterName is the default name of the Kind cluster
	// used by the the e2e framework.
	DefaultManagementClusterName = "mgmt"

	// DefaultCapiImage is the default value for Flags.CapiImage if the env var
	// CAPI_IMAGE is an empty string.
	DefaultCapiImage = "gcr.io/k8s-staging-cluster-api/cluster-api-controller:master"

	// DefaultCapiGitRef is the default value for Flags.CapiGitRef if the env var
	// CAPI_GIT_REF is an empty string.
	DefaultCapiGitRef = "master"

	// DefaultCertGitRef is the default value for Flags.CertGitRef if the env var
	// CERT_GIT_REF is an empty string.
	DefaultCertGitRef = "v0.11.1"

	// DefaultKubernetesVersion is the default value for Flags.KubernetesVersion
	// if the env var KUBERNETES_VERSION is an empty string.
	DefaultKubernetesVersion = "v1.16.2"
)

// Flags contains command-line flags used by the e2e framework.
var Flags struct {

	// DefaultManagementClusterName is the default name of the Kind cluster
	// used by the the e2e framework.
	//
	// Defaults to the env var MANAGEMENT_CLUSTER_NAME if not empty, otherwise
	// DefaultManagementClusterName.
	ManagementClusterName string

	// InfraImage is the image used to install the infrastructure provider
	// components.
	//
	// Defaults to the env var INFRA_IMAGE.
	InfraImage string

	// InfraNamespace is the namespace in which the infrastructure provider's
	// components are expected to be deployed.
	//
	// Defaults to the env var INFRA_NAMESPACE.
	InfraNamespace string

	// CapiImage is the image used to install the core CAPI components.
	//
	// Defaults to the env var CAPI_IMAGE if not empty, otherwise
	// DefaultCapiImage.
	CapiImage string

	// CapiGitRef is the git reference used to run remote kustomize against the
	// CAPI repository.
	//
	// Defaults to the env var CAPI_GIT_REF if not empty, otherwise
	// DefaultCapiGitRef.
	CapiGitRef string

	// CertGitRef is the git reference used to run remote kustomize against the
	// cert-manager repository.
	//
	// Defaults to the env var CERT_GIT_REF if not empty, otherwise
	// DefaultCertGitRef.
	CertGitRef string

	// KubernetesVersion is the version of Kubernetes to deploy when testing.
	//
	// Defaults to the env var KUBERNETES_VERSION if not empty, otherwise
	// DefaultKubernetesVersion.
	KubernetesVersion string
}

func init() {
	if Flags.ManagementClusterName = os.Getenv("MANAGEMENT_CLUSTER_NAME"); Flags.ManagementClusterName == "" {
		Flags.ManagementClusterName = DefaultManagementClusterName
	}
	if Flags.CapiImage = os.Getenv("CAPI_IMAGE"); Flags.CapiImage == "" {
		Flags.CapiImage = DefaultCapiImage
	}
	if Flags.CapiGitRef = os.Getenv("CAPI_GIT_REF"); Flags.CapiGitRef == "" {
		Flags.CapiGitRef = DefaultCapiGitRef
	}
	if Flags.CertGitRef = os.Getenv("CERT_GIT_REF"); Flags.CertGitRef == "" {
		Flags.CertGitRef = DefaultCertGitRef
	}
	if Flags.KubernetesVersion = os.Getenv("KUBERNETES_VERSION"); Flags.KubernetesVersion == "" {
		Flags.KubernetesVersion = DefaultKubernetesVersion
	}
	flag.StringVar(&Flags.ManagementClusterName, "e2e.managementClusterName", Flags.ManagementClusterName, "the name of the kind cluster used by the e2e framework")
	flag.StringVar(&Flags.InfraImage, "e2e.infraImage", os.Getenv("INFRA_IMAGE"), "the infrastructure provider's manager image")
	flag.StringVar(&Flags.InfraNamespace, "e2e.infraNamespace", os.Getenv("INFRA_NAMESPACE"), "the infrastructure provider's namespace")
	flag.StringVar(&Flags.CapiImage, "e2e.capiImage", Flags.CapiImage, "the capi manager image")
	flag.StringVar(&Flags.CapiGitRef, "e2e.capiGitRef", Flags.CapiGitRef, "the git reference used to run remote kustomize against CAPI")
	flag.StringVar(&Flags.CertGitRef, "e2e.certGitRef", Flags.CertGitRef, "the git reference used to run remote kustomize against cert-manager")
	flag.StringVar(&Flags.KubernetesVersion, "e2e.kubernetesVersion", Flags.KubernetesVersion, "the version of kubernetes to deploy")
}
