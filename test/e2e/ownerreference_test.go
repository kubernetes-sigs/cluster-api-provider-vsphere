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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

var _ = Describe("OwnerReference checks with FailureDomains and ClusterIdentity", func() {
	// Before running the test create the secret used by the VSphereClusterIdentity to connect to the vCenter.
	BeforeEach(func() {
		createVsphereIdentitySecret(ctx, bootstrapClusterProxy)
	})

	capi_e2e.QuickStartSpec(ctx, func() capi_e2e.QuickStartSpecInput {
		return capi_e2e.QuickStartSpecInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ArtifactFolder:        artifactFolder,
			SkipCleanup:           skipCleanup,
			Flavor:                pointer.String("ownerreferences"),
			PostMachinesProvisioned: func(proxy framework.ClusterProxy, namespace, clusterName string) {
				// Inject a client to use for checkClusterIdentitySecretOwnerRef
				checkClusterIdentitySecretOwnerRef(ctx, proxy.GetClient())

				// Set up a periodic patch to ensure the DeploymentZone is reconciled.
				forcePeriodicReconcile(ctx, proxy.GetClient(), namespace)

				// This check ensures that owner references are resilient - i.e. correctly re-reconciled - when removed.
				framework.ValidateOwnerReferencesResilience(ctx, proxy, namespace, clusterName,
					framework.CoreOwnerReferenceAssertion,
					framework.KubeadmBootstrapOwnerReferenceAssertions,
					framework.KubeadmControlPlaneOwnerReferenceAssertions,
					framework.ExpOwnerReferenceAssertions,
					VSphereKubernetesReferenceAssertions,
					VSphereReferenceAssertions,
				)
				// This check ensures that owner references are always updated to the most recent apiVersion.
				framework.ValidateOwnerReferencesOnUpdate(ctx, proxy, namespace, clusterName,
					framework.CoreOwnerReferenceAssertion,
					framework.KubeadmBootstrapOwnerReferenceAssertions,
					framework.KubeadmControlPlaneOwnerReferenceAssertions,
					framework.ExpOwnerReferenceAssertions,
					VSphereKubernetesReferenceAssertions,
					VSphereReferenceAssertions,
				)
			},
		}
	})

	// Delete objects created by the test which are not in the test namespace.
	AfterEach(func() {
		cleanupVSphereObjects(ctx, bootstrapClusterProxy)
	})

})

var (
	VSphereKubernetesReferenceAssertions = map[string]func([]metav1.OwnerReference) error{
		// Need custom Kubernetes assertions for secrets. Secrets in the CAPV tests can also be owned by the vSphereCluster.
		"Secret": func(owners []metav1.OwnerReference) error {
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
		"ConfigMap": func(owners []metav1.OwnerReference) error {
			// The only configMaps considered here are those owned by a ClusterResourceSet.
			return framework.HasExactOwners(owners, clusterResourceSetOwner)
		},
	}
)

var (
	VSphereReferenceAssertions = map[string]func([]metav1.OwnerReference) error{
		"VSphereCluster": func(owners []metav1.OwnerReference) error {
			return framework.HasExactOwners(owners, clusterController)
		},
		"VSphereClusterTemplate": func(owners []metav1.OwnerReference) error {
			return framework.HasExactOwners(owners, clusterClassOwner)
		},
		"VSphereMachine": func(owners []metav1.OwnerReference) error {
			return framework.HasExactOwners(owners, machineController)
		},
		"VSphereMachineTemplate": func(owners []metav1.OwnerReference) error {
			// The vSphereMachineTemplate can be owned by the Cluster or the ClusterClass.
			return framework.HasOneOfExactOwners(owners, []metav1.OwnerReference{clusterOwner}, []metav1.OwnerReference{clusterClassOwner})
		},
		"VSphereVM": func(owners []metav1.OwnerReference) error {
			return framework.HasExactOwners(owners, vSphereMachineOwner)
		},
		// VSphereClusterIdentity does not have any owners.
		"VSphereClusterIdentity": func(owners []metav1.OwnerReference) error {
			// The vSphereClusterIdentity does not have any owners.
			return framework.HasExactOwners(owners)
		},
		"VSphereDeploymentZone": func(owners []metav1.OwnerReference) error {
			// The vSphereDeploymentZone does not have any owners.
			return framework.HasExactOwners(owners)
		},
		"VSphereFailureDomain": func(owners []metav1.OwnerReference) error {
			// The vSphereFailureDomain can be owned by one or more vSphereDeploymentZones.
			return framework.HasOneOfExactOwners(owners, []metav1.OwnerReference{vSphereDeploymentZoneOwner}, []metav1.OwnerReference{vSphereDeploymentZoneOwner, vSphereDeploymentZoneOwner})
		},
	}
)

var (
	// CAPV owners.
	vSphereMachineOwner         = metav1.OwnerReference{Kind: "VSphereMachine", APIVersion: infrav1.GroupVersion.String()}
	vSphereClusterOwner         = metav1.OwnerReference{Kind: "VSphereCluster", APIVersion: infrav1.GroupVersion.String()}
	vSphereDeploymentZoneOwner  = metav1.OwnerReference{Kind: "VSphereDeploymentZone", APIVersion: infrav1.GroupVersion.String()}
	vSphereClusterIdentityOwner = metav1.OwnerReference{Kind: "VSphereClusterIdentity", APIVersion: infrav1.GroupVersion.String()}

	// CAPI owners.
	clusterClassOwner       = metav1.OwnerReference{Kind: "ClusterClass", APIVersion: clusterv1.GroupVersion.String()}
	clusterOwner            = metav1.OwnerReference{Kind: "Cluster", APIVersion: clusterv1.GroupVersion.String()}
	clusterController       = metav1.OwnerReference{Kind: "Cluster", APIVersion: clusterv1.GroupVersion.String(), Controller: pointer.Bool(true)}
	machineController       = metav1.OwnerReference{Kind: "Machine", APIVersion: clusterv1.GroupVersion.String(), Controller: pointer.Bool(true)}
	clusterResourceSetOwner = metav1.OwnerReference{Kind: "ClusterResourceSet", APIVersion: addonsv1.GroupVersion.String()}

	// KCP owner.
	kubeadmControlPlaneController = metav1.OwnerReference{Kind: "KubeadmControlPlane", APIVersion: controlplanev1.GroupVersion.String(), Controller: pointer.Bool(true)}

	// CAPBK owner.
	kubeadmConfigController = metav1.OwnerReference{Kind: "KubeadmConfig", APIVersion: bootstrapv1.GroupVersion.String(), Controller: pointer.Bool(true)}
)

// The following names are hardcoded in templates to make cleanup easier.
var (
	clusterIdentityName            = "ownerreferences"
	clusterIdentitySecretNamespace = "capv-system"
	deploymentZoneName             = "ownerreferences"
)

// cleanupVSphereObjects deletes the Secret, VSphereClusterIdentity, and VSphereDeploymentZone created for this test.
// The VSphereFailureDomain, and the Secret for the VSphereClusterIdentity should be deleted as a result of the above.
func cleanupVSphereObjects(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy) bool {
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
	return true
}

func createVsphereIdentitySecret(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy) {
	username := e2eConfig.GetVariable("VSPHERE_USERNAME")
	password := e2eConfig.GetVariable("VSPHERE_PASSWORD")
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

func checkClusterIdentitySecretOwnerRef(ctx context.Context, c ctrlclient.Client) {
	s := &corev1.Secret{}

	Eventually(func() error {
		if err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: clusterIdentitySecretNamespace, Name: clusterIdentityName}, s); err != nil {
			return err
		}
		return framework.HasExactOwners(s.GetOwnerReferences(), vSphereClusterIdentityOwner)
	}, 1*time.Minute).Should(Succeed())

	// Patch the secret to have a wrong APIVersion.
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
	Expect(helper.Patch(ctx, s)).To(Succeed())

	// Force reconcile the ClusterIdentity which owns the secret.
	annotationPatch := ctrlclient.RawPatch(types.MergePatchType, []byte(fmt.Sprintf("{\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/modifiedAt\":\"%v\"}}}", time.Now().Format(time.RFC3339))))
	Expect(c.Patch(ctx, &infrav1.VSphereClusterIdentity{ObjectMeta: metav1.ObjectMeta{Name: clusterIdentityName}}, annotationPatch)).To(Succeed())

	// Check that the secret ownerReferences were correctly reconciled.
	Eventually(func() error {
		if err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: clusterIdentitySecretNamespace, Name: clusterIdentityName}, s); err != nil {
			return err
		}
		return framework.HasExactOwners(s.GetOwnerReferences(), vSphereClusterIdentityOwner)
	}, 5*time.Minute).Should(Succeed())
}

// forcePeriodicReconcile forces the vSphereDeploymentZone and ClusterResourceSets to reconcile every 20 seconds.
// This reduces the chance of race conditions resulting in flakes in the test.
func forcePeriodicReconcile(ctx context.Context, c ctrlclient.Client, namespace string) {
	deploymentZoneList := &infrav1.VSphereDeploymentZoneList{}
	crsList := &addonsv1.ClusterResourceSetList{}
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
				Expect(c.List(ctx, crsList, ctrlclient.InNamespace(namespace))).To(Succeed())
				for _, crs := range crsList.Items {
					annotationPatch := ctrlclient.RawPatch(types.MergePatchType, []byte(fmt.Sprintf("{\"metadata\":{\"annotations\":{\"cluster.x-k8s.io/modifiedAt\":\"%v\"}}}", time.Now().Format(time.RFC3339))))
					Expect(c.Patch(ctx, crs.DeepCopy(), annotationPatch)).To(Succeed())
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
