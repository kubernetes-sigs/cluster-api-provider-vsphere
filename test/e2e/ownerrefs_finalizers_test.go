/*
Copyright 2023 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	clusterctlcluster "sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

var _ = Describe("Ensure OwnerReferences and Finalizers are resilient [vcsim] [supervisor]", func() {
	const specName = "owner-reference"
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
			return capi_e2e.QuickStartSpecInput{
				E2EConfig:             e2eConfig,
				ClusterctlConfigPath:  testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy: bootstrapClusterProxy,
				ArtifactFolder:        artifactFolder,
				SkipCleanup:           skipCleanup,
				Flavor:                ptr.To(testSpecificSettingsGetter().FlavorForMode("ownerrefs-finalizers")),
				PostNamespaceCreated: func(managementClusterProxy framework.ClusterProxy, workloadClusterNamespace string) {
					testSpecificSettingsGetter().PostNamespaceCreatedFunc(managementClusterProxy, workloadClusterNamespace)

					// The PostNamespaceCreatedFunc adds the VSPHERE_USERNAME and VSPHERE_PASSWORD
					// when testing on VCSim so we can only set them after running it.
					if testMode == GovmomiTestMode {
						// NOTE: When testing with vcsim VSPHERE_USERNAME and VSPHERE_PASSWORD are provided as a test specific variables,
						// when running on CI same variables are provided as env variables.
						input := testSpecificSettingsGetter()
						username, ok := input.Variables["VSPHERE_USERNAME"]
						if !ok {
							username = os.Getenv("VSPHERE_USERNAME")
						}
						password, ok := input.Variables["VSPHERE_PASSWORD"]
						if !ok {
							password = os.Getenv("VSPHERE_PASSWORD")
						}

						// Before running the test create the secret used by the VSphereClusterIdentity to connect to the vCenter.
						createVsphereIdentitySecret(ctx, bootstrapClusterProxy, username, password)
					}
				},
				PostMachinesProvisioned: func(proxy framework.ClusterProxy, namespace, clusterName string) {
					forceCtx, forceCancelFunc := context.WithCancel(ctx)
					if testMode == GovmomiTestMode {
						// check the cluster identity secret has expected ownerReferences and finalizers, and they are resilient
						// Note: identity secret is not part of the object graph, so it requires an ad-hoc test.
						checkClusterIdentitySecretOwnerRefAndFinalizer(ctx, proxy.GetClient())

						// Set up a periodic patch to ensure the DeploymentZone are reconciled.
						// Note: this is required because DeploymentZone are not watching for clusters, and thus the DeploymentZone controller
						// won't be triggered when we un-pause clusters after modifying objects ownerReferences & Finalizers to test resilience.
						forcePeriodicDeploymentZoneReconcile(forceCtx, proxy.GetClient())
					}

					// This check ensures that owner references are resilient - i.e. correctly re-reconciled - when removed.
					By("Checking that owner references are resilient")
					framework.ValidateOwnerReferencesResilience(ctx, proxy, namespace, clusterName, clusterctlcluster.FilterClusterObjectsWithNameFilter(clusterName),
						framework.CoreOwnerReferenceAssertion,
						framework.KubeadmBootstrapOwnerReferenceAssertions,
						framework.KubeadmControlPlaneOwnerReferenceAssertions,
						framework.ExpOwnerReferenceAssertions,
						VSphereKubernetesReferenceAssertions,
						VSphereReferenceAssertions(),
					)
					// This check ensures that owner references are always updated to the most recent apiVersion.
					By("Checking that owner references are updated to the correct API version")
					framework.ValidateOwnerReferencesOnUpdate(ctx, proxy, namespace, clusterName, clusterctlcluster.FilterClusterObjectsWithNameFilter(clusterName),
						framework.CoreOwnerReferenceAssertion,
						framework.KubeadmBootstrapOwnerReferenceAssertions,
						framework.KubeadmControlPlaneOwnerReferenceAssertions,
						framework.ExpOwnerReferenceAssertions,
						VSphereKubernetesReferenceAssertions,
						VSphereReferenceAssertions(),
					)
					// This check ensures that finalizers are resilient - i.e. correctly re-reconciled, when removed.
					// Note: we are not checking finalizers on VirtualMachine (finalizers are added by VM-Operator / vcsim controller)
					// as well as other VM Operator related kinds.
					By("Checking that finalizers are resilient")
					framework.ValidateFinalizersResilience(ctx, proxy, namespace, clusterName, FilterObjectsWithKindAndName(clusterName),
						framework.CoreFinalizersAssertionWithLegacyClusters,
						framework.KubeadmControlPlaneFinalizersAssertion,
						framework.ExpFinalizersAssertion,
						vSphereFinalizers(),
					)

					// Stop periodic patch if any.
					if forceCancelFunc != nil {
						forceCancelFunc()
					}

					// This check ensures that the rollout to the machines finished before
					// checking resource version stability.
					Eventually(func() error {
						machineList := &clusterv1.MachineList{}
						if err := proxy.GetClient().List(ctx, machineList, ctrlclient.InNamespace(namespace)); err != nil {
							return errors.Wrap(err, "list machines")
						}

						for _, machine := range machineList.Items {
							if !conditions.IsTrue(&machine, clusterv1.MachineNodeHealthyCondition) {
								return errors.Errorf("machine %q does not have %q condition set to true", machine.GetName(), clusterv1.MachineNodeHealthyCondition)
							}
						}

						return nil
					}, 5*time.Minute, 15*time.Second).Should(Succeed(), "Waiting for nodes to be ready")

					// Dump all Cluster API related resources to artifacts before checking for resource versions being stable.
					framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
						Lister:    proxy.GetClient(),
						Namespace: namespace,
						LogPath:   filepath.Join(artifactFolder, "clusters-beforeValidateResourceVersions", proxy.GetName(), "resources")})

					// This check ensures that the resourceVersions are stable, i.e. it verifies there are no
					// continuous reconciles when everything should be stable.
					// Note: we are not checking resourceVersions on VirtualMachine (reconciled by VM-Operator)
					// as well as other VM Operator related kinds.
					By("Checking that resourceVersions are stable")
					framework.ValidateResourceVersionStable(ctx, proxy, namespace, FilterObjectsWithKindAndName(clusterName))
				},
			}
		})

		// Delete objects created by the test which are not in the test namespace.
		AfterEach(func() {
			if testMode == GovmomiTestMode {
				cleanupVSphereObjects(ctx, bootstrapClusterProxy)
			}
		})
	})
})

var (
	VSphereKubernetesReferenceAssertions = map[string]func(types.NamespacedName, []metav1.OwnerReference) error{
		// Need custom Kubernetes assertions for secrets. Secrets in the CAPV tests can also be owned by the vSphereCluster.
		"Secret": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
			return framework.HasOneOfExactOwners(owners,
				// Secrets for cluster certificates must be owned by the KubeadmControlPlane.
				[]metav1.OwnerReference{kubeadmControlPlaneController},
				// The bootstrap secret should be owned by a KubeadmConfig.
				[]metav1.OwnerReference{kubeadmConfigController},
				// Secrets created as a resource for a ClusterResourceSet can be owned by the ClusterResourceSet.
				[]metav1.OwnerReference{clusterResourceSetOwner},
				// Secrets created as an identityReference for a vSphereCluster should be owned but the vSphereCluster.
				[]metav1.OwnerReference{vSphereClusterOwner},
			)
		},
		"ConfigMap": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
			// The only configMaps considered here are those owned by a ClusterResourceSet.
			return framework.HasExactOwners(owners, clusterResourceSetOwner)
		},
	}
)

var (
	VSphereReferenceAssertions = func() map[string]func(types.NamespacedName, []metav1.OwnerReference) error {
		if testMode == SupervisorTestMode {
			return map[string]func(types.NamespacedName, []metav1.OwnerReference) error{
				"VSphereCluster": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
					return framework.HasExactOwners(owners, clusterController)
				},
				"VSphereClusterTemplate": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
					return framework.HasExactOwners(owners, clusterClassOwner)
				},
				"VSphereMachine": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
					return framework.HasExactOwners(owners, machineController)
				},
				"VSphereMachineTemplate": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
					// The vSphereMachineTemplate can be owned by the Cluster or the ClusterClass.
					return framework.HasOneOfExactOwners(owners, []metav1.OwnerReference{clusterOwner}, []metav1.OwnerReference{clusterClassOwner})
				},
				"VirtualMachine": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
					return framework.HasExactOwners(owners, vmwareVSphereMachineController)
				},

				// Following objects are for vm-operator (not managed by CAPV), so checking ownerReferences is not relevant.
				"VirtualMachineImage":             func(_ types.NamespacedName, _ []metav1.OwnerReference) error { return nil },
				"NetworkInterface":                func(_ types.NamespacedName, _ []metav1.OwnerReference) error { return nil },
				"ContentSourceBinding":            func(_ types.NamespacedName, _ []metav1.OwnerReference) error { return nil },
				"VirtualMachineSetResourcePolicy": func(_ types.NamespacedName, _ []metav1.OwnerReference) error { return nil },
				"VirtualMachineClassBinding":      func(_ types.NamespacedName, _ []metav1.OwnerReference) error { return nil },
				"VirtualMachineClass":             func(_ types.NamespacedName, _ []metav1.OwnerReference) error { return nil },
				"VMOperatorDependencies":          func(_ types.NamespacedName, _ []metav1.OwnerReference) error { return nil },
			}
		}

		return map[string]func(types.NamespacedName, []metav1.OwnerReference) error{
			"VSphereCluster": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				return framework.HasExactOwners(owners, clusterController)
			},
			"VSphereClusterTemplate": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				return framework.HasExactOwners(owners, clusterClassOwner)
			},
			"VSphereMachine": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				return framework.HasExactOwners(owners, machineController)
			},
			"VSphereMachineTemplate": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				// The vSphereMachineTemplate can be owned by the Cluster or the ClusterClass.
				return framework.HasOneOfExactOwners(owners, []metav1.OwnerReference{clusterOwner}, []metav1.OwnerReference{clusterClassOwner})
			},
			"VSphereVM": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				return framework.HasExactOwners(owners, vSphereMachineOwner)
			},
			// VSphereClusterIdentity does not have any owners.
			"VSphereClusterIdentity": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				// The vSphereClusterIdentity does not have any owners.
				return framework.HasExactOwners(owners)
			},
			"VSphereDeploymentZone": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				// The vSphereDeploymentZone does not have any owners.
				return framework.HasExactOwners(owners)
			},
			"VSphereFailureDomain": func(_ types.NamespacedName, owners []metav1.OwnerReference) error {
				// The vSphereFailureDomain can be owned by one or more vSphereDeploymentZones.
				return framework.HasOneOfExactOwners(owners, []metav1.OwnerReference{vSphereDeploymentZoneOwner}, []metav1.OwnerReference{vSphereDeploymentZoneOwner, vSphereDeploymentZoneOwner})
			},
		}
	}
)

var (
	// CAPV owners.
	vSphereMachineOwner         = metav1.OwnerReference{Kind: "VSphereMachine", APIVersion: infrav1.GroupVersion.String()}
	vSphereClusterOwner         = metav1.OwnerReference{Kind: "VSphereCluster", APIVersion: infrav1.GroupVersion.String()}
	vSphereDeploymentZoneOwner  = metav1.OwnerReference{Kind: "VSphereDeploymentZone", APIVersion: infrav1.GroupVersion.String()}
	vSphereClusterIdentityOwner = metav1.OwnerReference{Kind: "VSphereClusterIdentity", APIVersion: infrav1.GroupVersion.String()}

	vmwareVSphereMachineController = metav1.OwnerReference{Kind: "VSphereMachine", APIVersion: vmwarev1.GroupVersion.String(), Controller: ptr.To(true)}

	// CAPI owners.
	clusterClassOwner       = metav1.OwnerReference{Kind: "ClusterClass", APIVersion: clusterv1.GroupVersion.String()}
	clusterOwner            = metav1.OwnerReference{Kind: "Cluster", APIVersion: clusterv1.GroupVersion.String()}
	clusterController       = metav1.OwnerReference{Kind: "Cluster", APIVersion: clusterv1.GroupVersion.String(), Controller: ptr.To(true)}
	machineController       = metav1.OwnerReference{Kind: "Machine", APIVersion: clusterv1.GroupVersion.String(), Controller: ptr.To(true)}
	clusterResourceSetOwner = metav1.OwnerReference{Kind: "ClusterResourceSet", APIVersion: addonsv1.GroupVersion.String()}

	// KCP owner.
	kubeadmControlPlaneController = metav1.OwnerReference{Kind: "KubeadmControlPlane", APIVersion: controlplanev1.GroupVersion.String(), Controller: ptr.To(true)}

	// CAPBK owner.
	kubeadmConfigController = metav1.OwnerReference{Kind: "KubeadmConfig", APIVersion: bootstrapv1.GroupVersion.String(), Controller: ptr.To(true)}
)

// The following names are hardcoded in templates to make cleanup easier.
var (
	clusterIdentityName            = "ownerrefs-finalizers"
	clusterIdentitySecretNamespace = "capv-system"
	deploymentZoneName             = "ownerrefs-finalizers"
)

// vSphereFinalizers maps VSphere infrastructure resource types to their expected finalizers.
var vSphereFinalizers = func() map[string]func(types.NamespacedName) []string {
	if testMode == SupervisorTestMode {
		return map[string]func(types.NamespacedName) []string{
			"VSphereMachine": func(_ types.NamespacedName) []string { return []string{infrav1.MachineFinalizer} },
			"VSphereCluster": func(_ types.NamespacedName) []string { return []string{vmwarev1.ClusterFinalizer} },
		}
	}

	return map[string]func(types.NamespacedName) []string{
		"VSphereVM": func(_ types.NamespacedName) []string {
			// When using vcsim additional finalizers are added.
			if testTarget == VCSimTestTarget {
				return []string{infrav1.VMFinalizer, vcsimv1.VMFinalizer}
			}
			return []string{infrav1.VMFinalizer}
		},
		"VSphereClusterIdentity": func(_ types.NamespacedName) []string { return []string{infrav1.VSphereClusterIdentityFinalizer} },
		"VSphereDeploymentZone":  func(_ types.NamespacedName) []string { return []string{infrav1.DeploymentZoneFinalizer} },
		"VSphereMachine":         func(_ types.NamespacedName) []string { return []string{infrav1.MachineFinalizer} },
		"IPAddressClaim":         func(_ types.NamespacedName) []string { return []string{infrav1.IPAddressClaimFinalizer} },
		"VSphereCluster":         func(_ types.NamespacedName) []string { return []string{infrav1.ClusterFinalizer} },
	}
}

// cleanupVSphereObjects deletes the Secret, VSphereClusterIdentity, and VSphereDeploymentZone created for this test.
// The VSphereFailureDomain, and the Secret for the VSphereClusterIdentity should be deleted as a result of the above.
func cleanupVSphereObjects(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy) {
	Eventually(func() error {
		if err := bootstrapClusterProxy.GetClient().Delete(ctx,
			&infrav1.VSphereClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterIdentityName,
				},
			}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if err := bootstrapClusterProxy.GetClient().Delete(ctx,
			&infrav1.VSphereDeploymentZone{
				ObjectMeta: metav1.ObjectMeta{
					Name: deploymentZoneName,
				},
			}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}).Should(Succeed())
}

func createVsphereIdentitySecret(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, username, password string) {
	Expect(username).To(Not(BeEmpty()))
	Expect(password).To(Not(BeEmpty()))
	Expect(bootstrapClusterProxy.GetClient().Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: clusterIdentitySecretNamespace,
				Name:      clusterIdentityName,
			},
			Data: map[string][]byte{
				"password": []byte(password),
				"username": []byte(username),
			},
		})).To(Succeed())
}

func checkClusterIdentitySecretOwnerRefAndFinalizer(ctx context.Context, c ctrlclient.Client) {
	s := &corev1.Secret{}

	By("Check that the ownerReferences and finalizers for the ClusterIdentitySecret are as expected")
	Eventually(func() error {
		if err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: clusterIdentitySecretNamespace, Name: clusterIdentityName}, s); err != nil {
			return err
		}
		if err := framework.HasExactOwners(s.GetOwnerReferences(), vSphereClusterIdentityOwner); err != nil {
			return err
		}
		if !sets.NewString(s.GetFinalizers()...).Equal(sets.NewString(infrav1.SecretIdentitySetFinalizer)) {
			return errors.Errorf("the ClusterIdentitySecret %s does not have finalizers", klog.KRef(clusterIdentitySecretNamespace, clusterIdentityName))
		}
		return nil
	}, 1*time.Minute).Should(Succeed())

	// Patch the secret to have a wrong APIVersion in ownerRef and to remove finalizers.
	By("Removing all the ownerReferences and finalizers for the ClusterIdentitySecret")
	helper, err := patch.NewHelper(s, c)
	Expect(err).ToNot(HaveOccurred())
	newOwners := []metav1.OwnerReference{}
	for _, owner := range s.GetOwnerReferences() {
		var gv schema.GroupVersion
		gv, err := schema.ParseGroupVersion(owner.APIVersion)
		Expect(err).ToNot(HaveOccurred())
		gv.Version = "v1alpha1"
		owner.APIVersion = gv.String()
		newOwners = append(newOwners, owner)
	}
	s.SetOwnerReferences(newOwners)
	s.SetFinalizers(nil)
	Expect(helper.Patch(ctx, s)).To(Succeed())

	// Force reconcile the ClusterIdentity which owns the secret.
	annotationPatch := ctrlclient.RawPatch(types.MergePatchType, []byte(fmt.Sprintf("{\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/modifiedAt\":\"%v\"}}}", time.Now().Format(time.RFC3339))))
	Expect(c.Patch(ctx, &infrav1.VSphereClusterIdentity{ObjectMeta: metav1.ObjectMeta{Name: clusterIdentityName}}, annotationPatch)).To(Succeed())

	// Check that the secret ownerReferences were correctly reconciled.
	By("Check that the ownerReferences and finalizers for the ClusterIdentitySecret are rebuilt as expected")
	Eventually(func() error {
		if err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: clusterIdentitySecretNamespace, Name: clusterIdentityName}, s); err != nil {
			return err
		}
		if err := framework.HasExactOwners(s.GetOwnerReferences(), vSphereClusterIdentityOwner); err != nil {
			return err
		}
		if !sets.NewString(s.GetFinalizers()...).Equal(sets.NewString(infrav1.SecretIdentitySetFinalizer)) {
			return errors.Errorf("the ClusterIdentitySecret %s does not have finalizers", klog.KRef(clusterIdentitySecretNamespace, clusterIdentityName))
		}
		return nil
	}, 5*time.Minute).Should(Succeed())
}

// forcePeriodicDeploymentZoneReconcile forces the vSphereDeploymentZone to reconcile every 20 seconds.
// This reduces the chance of race conditions resulting in flakes in the test.
func forcePeriodicDeploymentZoneReconcile(ctx context.Context, c ctrlclient.Client) {
	deploymentZoneList := &infrav1.VSphereDeploymentZoneList{}
	ticker := time.NewTicker(20 * time.Second)
	stopTimer := time.NewTimer(5 * time.Minute)
	go func() {
		defer GinkgoRecover()
		for {
			select {
			case <-ticker.C:
				Expect(c.List(ctx, deploymentZoneList)).To(Succeed())
				for _, zone := range deploymentZoneList.Items {
					annotationPatch := ctrlclient.RawPatch(types.MergePatchType, []byte(fmt.Sprintf("{\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/modifiedAt\":\"%v\"}}}", time.Now().Format(time.RFC3339))))
					Expect(c.Patch(ctx, zone.DeepCopy(), annotationPatch)).To(Succeed())
				}
			case <-stopTimer.C:
				ticker.Stop()
				return
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func FilterObjectsWithKindAndName(clusterName string) func(u unstructured.Unstructured) bool {
	f := clusterctlcluster.FilterClusterObjectsWithNameFilter(clusterName)

	return func(u unstructured.Unstructured) bool {
		// Following objects are for vm-operator (not managed by CAPV), so checking finalizers/resourceVersion is not relevant.
		// Note: we are excluding also VirtualMachines, which instead are considered for the ownerReference tests.
		if testMode == SupervisorTestMode {
			if sets.NewString("VirtualMachineImage", "NetworkInterface", "ContentSourceBinding", "VirtualMachineSetResourcePolicy", "VirtualMachineClass", "VMOperatorDependencies", "VirtualMachine").Has(u.GetKind()) {
				return false
			}
		}
		return f(u)
	}
}
