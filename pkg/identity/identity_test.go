/*
Copyright 2021 The Kubernetes Authors.

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

package identity

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

// nolint:goconst
var _ = Describe("GetCredentials", func() {
	var (
		ns      *corev1.Namespace
		cluster *infrav1.VSphereCluster
	)

	BeforeEach(func() {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "namespace-",
			},
		}
		Expect(k8sclient.Create(ctx, ns)).To(Succeed())

		cluster = &infrav1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "cluster-",
				Namespace:    ns.Name,
			},
		}

		Expect(k8sclient.Create(ctx, cluster)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sclient.Delete(ctx, ns)).To(Succeed())
	})

	Context("with using a secret directly", func() {
		It("should find and return credentials from a secret within same namespace", func() {
			credentialSecret := createSecret(cluster.Namespace)
			cluster.Spec = infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.SecretKind,
					Name: credentialSecret.Name,
				},
			}
			Expect(k8sclient.Update(ctx, cluster)).To(Succeed())
			creds, err := GetCredentials(ctx, k8sclient, cluster, manager.DefaultPodNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(creds.Username).To(Equal(getData(credentialSecret, UsernameKey)))
			Expect(creds.Password).To(Equal(getData(credentialSecret, PasswordKey)))
		})

		It("should error if secret is not in the same namespace as the cluster", func() {
			credentialSecret := createSecret(manager.DefaultPodNamespace)
			cluster.Spec = infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.SecretKind,
					Name: credentialSecret.Name,
				},
			}
			Expect(k8sclient.Update(ctx, cluster)).To(Succeed())

			_, err := GetCredentials(ctx, k8sclient, cluster, manager.DefaultPodNamespace)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with using a VSphereClusterIdentity", func() {
		It("should fetch the secret from the controller namespace and return credentials", func() {
			credentialSecret := createSecret(manager.DefaultPodNamespace)
			identity := createIdentity(credentialSecret.Name)

			labels := ns.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			labels["identity-authorized"] = "true"
			ns.Labels = labels
			Expect(k8sclient.Update(ctx, ns)).To(Succeed())

			cluster.Spec = infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.VSphereClusterIdentityKind,
					Name: identity.Name,
				},
			}
			Expect(k8sclient.Update(ctx, cluster)).To(Succeed())

			creds, err := GetCredentials(ctx, k8sclient, cluster, manager.DefaultPodNamespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(creds.Username).To(Equal(getData(credentialSecret, UsernameKey)))
			Expect(creds.Password).To(Equal(getData(credentialSecret, PasswordKey)))
		})

		It("should error if allowedNamespaces is set to nil", func() {
			credentialSecret := createSecret(manager.DefaultPodNamespace)
			identity := createIdentity(credentialSecret.Name)
			// set allowedNamespaces to nil and update
			identity.Spec.AllowedNamespaces = nil
			Expect(k8sclient.Update(ctx, identity)).To(Succeed())

			labels := ns.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			labels["identity-authorized"] = "true"
			ns.Labels = labels
			Expect(k8sclient.Update(ctx, ns)).To(Succeed())

			cluster.Spec = infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.VSphereClusterIdentityKind,
					Name: identity.Name,
				},
			}
			Expect(k8sclient.Update(ctx, cluster)).To(Succeed())

			_, err := GetCredentials(ctx, k8sclient, cluster, manager.DefaultPodNamespace)
			Expect(err).To(HaveOccurred())
		})

		It("should error if the selector does not match the target namespace", func() {
			credentialSecret := createSecret(manager.DefaultPodNamespace)
			identity := createIdentity(credentialSecret.Name)

			cluster.Spec = infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.VSphereClusterIdentityKind,
					Name: identity.Name,
				},
			}
			Expect(k8sclient.Update(ctx, cluster)).To(Succeed())

			_, err := GetCredentials(ctx, k8sclient, cluster, manager.DefaultPodNamespace)
			Expect(err).To(HaveOccurred())
		})

		It("should error if identity isn't Ready", func() {
			credentialSecret := createSecret(manager.DefaultPodNamespace)
			identity := createIdentity(credentialSecret.Name)
			identity.Status.Ready = false
			Expect(k8sclient.Status().Update(ctx, identity)).To(Succeed())

			labels := ns.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			labels["identity-authorized"] = "true"
			ns.Labels = labels
			Expect(k8sclient.Update(ctx, ns)).To(Succeed())

			cluster.Spec = infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.VSphereClusterIdentityKind,
					Name: identity.Name,
				},
			}
			Expect(k8sclient.Update(ctx, cluster)).To(Succeed())

			_, err := GetCredentials(ctx, k8sclient, cluster, manager.DefaultPodNamespace)

			Expect(err).To(HaveOccurred())
		})
	})

	Context("prerequisites missing", func() {
		It("should error if cluster is missing", func() {
			_, err := GetCredentials(ctx, k8sclient, nil, manager.DefaultPodNamespace)
			Expect(err).To(HaveOccurred())
		})

		It("should error if client is missing", func() {
			_, err := GetCredentials(ctx, nil, cluster, manager.DefaultPodNamespace)
			Expect(err).To(HaveOccurred())
		})

		It("should error if identityRef is missing on cluster", func() {
			_, err := GetCredentials(ctx, k8sclient, cluster, manager.DefaultPodNamespace)
			Expect(err).To(HaveOccurred())
		})
	})
})

func createSecret(namespace string) *corev1.Secret {
	credentialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "secret-",
			Namespace:    namespace,
		},
		Data: map[string][]byte{
			UsernameKey: []byte("user"),
			PasswordKey: []byte("pass"),
		},
	}
	Expect(k8sclient.Create(ctx, credentialSecret)).To(Succeed())
	return credentialSecret
}

func createIdentity(secretName string) *infrav1.VSphereClusterIdentity {
	identity := &infrav1.VSphereClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "identity-",
		},
		Spec: infrav1.VSphereClusterIdentitySpec{
			SecretName: secretName,
			AllowedNamespaces: &infrav1.AllowedNamespaces{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"identity-authorized": "true",
					},
				},
			},
		},
	}
	Expect(k8sclient.Create(ctx, identity)).To(Succeed())
	identity.Status.Ready = true
	Expect(k8sclient.Status().Update(ctx, identity)).To(Succeed())
	return identity
}
