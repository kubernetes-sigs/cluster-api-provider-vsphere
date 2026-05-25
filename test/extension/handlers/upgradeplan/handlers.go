/*
Copyright 2026 The Kubernetes Authors.

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

// Package upgradeplan contains the handlers for the upgrade plan hook.
//
// The implementation of the handlers is specifically designed for Cluster API provider vSphere E2E tests use cases.
// When implementing custom RuntimeExtension, it is only required to expose HandlerFunc with the
// signature defined in sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1.
package upgradeplan

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"k8s.io/klog/v2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/topology/desiredstate"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExtensionHandlers provides a common struct for the upgrade plan hook handler.
// NOTE: it is not mandatory to use a ExtensionHandlers in custom RuntimeExtension, what is important
// is to expose HandlerFunc with the signature defined in sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1.
type ExtensionHandlers struct {
	client client.Client
}

// NewExtensionHandlers returns a ExtensionHandlers for the upgrade plan hook handler.
func NewExtensionHandlers(client client.Client) *ExtensionHandlers {
	return &ExtensionHandlers{
		client: client,
	}
}

// DoGenerateUpgradePlan implements the HandlerFunc for the GenerateUpgradePlan hook.
// This hook generates an upgrade plan based on the fact that CAPV maintainers are publishing a .0 image for each Kubernetes version.
// NOTE: custom RuntimeExtension, must implement the body of this func according to the specific use case.
func (m *ExtensionHandlers) DoGenerateUpgradePlan(ctx context.Context, request *runtimehooksv1.GenerateUpgradePlanRequest, response *runtimehooksv1.GenerateUpgradePlanResponse) {
	log := ctrl.LoggerFrom(ctx).WithValues("Cluster", klog.KObj(&request.Cluster))
	ctx = ctrl.LoggerInto(ctx, log)
	log.Info("GenerateUpgradePlan is called")

	// Get a list of Kubernetes versions for which we know image exists.
	versions := getKubernetesVersions(request.FromControlPlaneKubernetesVersion, request.FromWorkersKubernetesVersion, request.ToKubernetesVersion)

	// Use GetUpgradePlanFromClusterClassVersions to generate the upgrade plan.
	getUpgradePlan := desiredstate.GetUpgradePlanFromClusterClassVersions(versions)

	// Call the upgrade plan function.
	controlPlaneUpgrades, workersUpgrades, err := getUpgradePlan(ctx, request.ToKubernetesVersion, request.FromControlPlaneKubernetesVersion, request.FromWorkersKubernetesVersion)
	if err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}

	// Convert the upgrade plans to UpgradeStep format.
	response.ControlPlaneUpgrades = make([]runtimehooksv1.UpgradeStep, len(controlPlaneUpgrades))
	for i, version := range controlPlaneUpgrades {
		response.ControlPlaneUpgrades[i] = runtimehooksv1.UpgradeStep{
			Version: version,
		}
	}

	response.WorkersUpgrades = make([]runtimehooksv1.UpgradeStep, len(workersUpgrades))
	for i, version := range workersUpgrades {
		response.WorkersUpgrades[i] = runtimehooksv1.UpgradeStep{
			Version: version,
		}
	}

	response.Status = runtimehooksv1.ResponseStatusSuccess
}

func getKubernetesVersions(fromControlPlane, fromWorkers, to string) []string {
	fromMinor := semver.MustParse(strings.TrimPrefix(fromControlPlane, "v")).Minor
	if fromWorkers != "" {
		fromMinor = min(fromMinor, semver.MustParse(strings.TrimPrefix(fromWorkers, "v")).Minor)
	}
	toMinor := semver.MustParse(strings.TrimPrefix(to, "v")).Minor

	versions := []string{}
	for i := fromMinor + 1; i <= toMinor; i++ {
		if i == toMinor {
			versions = append(versions, to)
			break
		}
		versions = append(versions, fmt.Sprintf("v1.%d.0", i))
	}
	return versions
}
