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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/manager"
)

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
	Context("with checking if cluster spec identity Ref secret", func() {
		It("should return false if cluster is nil", func() {
			cluster = &infrav1.VSphereCluster{}
			Expect(IsSecretIdentity(cluster)).To(BeFalse())
		})
		It("should return false if cluster spec identity Ref is nil", func() {
			cluster.Spec = infrav1.VSphereClusterSpec{}
			Expect(IsSecretIdentity(cluster)).To(BeFalse())
		})
		It("should return true if cluster spec identity Ref is not nil", func() {
			credentialSecret := createSecret(cluster.Namespace)
			cluster.Spec = infrav1.VSphereClusterSpec{
				IdentityRef: &infrav1.VSphereIdentityReference{
					Kind: infrav1.SecretKind,
					Name: credentialSecret.Name,
				},
			}
			Expect(IsSecretIdentity(cluster)).To(BeTrue())
		})
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
})

var _ = Describe("validateInputs", func() {
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

	Context("If the client is missing", func() {
		It("should error if client is missing", func() {
			Expect(validateInputs(nil, cluster)).NotTo(Succeed())
		})
	})

	Context("If the cluster is missing", func() {
		It("should error if cluster is missing", func() {
			Expect(validateInputs(k8sclient, nil)).NotTo(Succeed())
		})
	})

	Context("If the identityRef is missing on cluster", func() {
		It("should error if identityRef is missing on cluster", func() {
			Expect(validateInputs(k8sclient, cluster)).NotTo(Succeed())
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

func TestIsOwnedByIdentityOrCluster(t *testing.T) {
	type args struct {
		ownerReferences []metav1.OwnerReference
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "No Owners",
			args: args{
				ownerReferences: []metav1.OwnerReference{},
			},
			want: false,
		},
		{
			name: "Owned by external entity",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "api",
						Kind:       "bla",
						Name:       "test",
					},
				},
			},
			want: false,
		},
		{
			name: "Owned by external different entity",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "api2",
						Kind:       "bla2",
						Name:       "tes2t",
					},
				},
			},
			want: false,
		},
		{
			name: "Owned by VSphereCluster/v1beta1",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
						Kind:       "VSphereCluster",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
		{
			name: "Owned by VSphereClusterIdentity/v1beta1",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
						Kind:       "VSphereClusterIdentity",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
		{
			name: "Owned by VSphereCluster/v1alpha4",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						Kind:       "VSphereCluster",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
		{
			name: "Owned by VSphereClusterIdentity/v1alpha4",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
						Kind:       "VSphereClusterIdentity",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
		{
			name: "Owned by VSphereCluster/v1alpha3",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						Kind:       "VSphereCluster",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
		{
			name: "Owned by VSphereClusterIdentity/v1alpha3",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						Kind:       "VSphereClusterIdentity",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
		{
			name: "Owned by vmware.infrastructure.cluster.x-k8s.io/VSphereCluster",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "vmware.infrastructure.cluster.x-k8s.io/v1beta1",
						Kind:       "VSphereCluster",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
		{
			name: "Owned by vmware.infrastructure.cluster.x-k8s.io/VSphereClusterIdentity",
			args: args{
				ownerReferences: []metav1.OwnerReference{
					{
						APIVersion: "vmware.infrastructure.cluster.x-k8s.io/v1beta1",
						Kind:       "VSphereClusterIdentity",
						Name:       "tes2t",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOwnedByIdentityOrCluster(tt.args.ownerReferences); got != tt.want {
				t.Errorf("IsOwnedByIdentityOrCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
