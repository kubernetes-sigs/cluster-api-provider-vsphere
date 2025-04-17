/*
Copyright 2025 The Kubernetes Authors.

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

package webhooks

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/cluster-api-provider-vsphere/internal/webhooks"
)

// VSphereClusterTemplateWebhook implements a validation and defaulting webhook for VSphereClusterTemplate.
type VSphereClusterTemplateWebhook struct{}

// SetupWebhookWithManager sets up VSphereClusterTemplate webhooks.
func (webhook *VSphereClusterTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return (&webhooks.VSphereClusterTemplateWebhook{}).SetupWebhookWithManager(mgr)
}

// VSphereDeploymentZoneWebhook implements a validation and defaulting webhook for VSphereDeploymentZone.
type VSphereDeploymentZoneWebhook struct{}

// SetupWebhookWithManager sets up VSphereDeploymentZone webhooks.
func (webhook *VSphereDeploymentZoneWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return (&webhooks.VSphereDeploymentZoneWebhook{}).SetupWebhookWithManager(mgr)
}

// VSphereFailureDomainWebhook implements a validation and defaulting webhook for VSphereFailureDomain.
type VSphereFailureDomainWebhook struct{}

// SetupWebhookWithManager sets up VSphereFailureDomain webhooks.
func (webhook *VSphereFailureDomainWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return (&webhooks.VSphereFailureDomainWebhook{}).SetupWebhookWithManager(mgr)
}

// VSphereMachineWebhook implements a validation and defaulting webhook for VSphereMachine.
type VSphereMachineWebhook struct{}

// SetupWebhookWithManager sets up VSphereMachine webhooks.
func (webhook *VSphereMachineWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return (&webhooks.VSphereMachineWebhook{}).SetupWebhookWithManager(mgr)
}

// VSphereMachineTemplateWebhook implements a validation and defaulting webhook for VSphereMachineTemplate.
type VSphereMachineTemplateWebhook struct{}

// SetupWebhookWithManager sets up VSphereMachineTemplate webhooks.
func (webhook *VSphereMachineTemplateWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return (&webhooks.VSphereMachineTemplateWebhook{}).SetupWebhookWithManager(mgr)
}

// VSphereVMWebhook implements a validation and defaulting webhook for VSphereVM.
type VSphereVMWebhook struct{}

// SetupWebhookWithManager sets up VSphereVM webhooks.
func (webhook *VSphereVMWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return (&webhooks.VSphereVMWebhook{}).SetupWebhookWithManager(mgr)
}
