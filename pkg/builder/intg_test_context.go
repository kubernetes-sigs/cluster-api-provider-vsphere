/*
Copyright 2022 The Kubernetes Authors.

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
package builder

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
)

// IntegrationTestContext is used for integration testing
// Supervisor controllers.
type IntegrationTestContext struct {
	context.Context
	Client            client.Client
	GuestClient       client.Client
	Namespace         string
	Cluster           *clusterv1.Cluster
	ClusterKey        client.ObjectKey
	VSphereCluster    *vmwarev1.VSphereCluster
	VSphereClusterKey client.ObjectKey
	envTest           *envtest.Environment
	suite             *TestSuite
	PatchHelper       *patch.Helper
}

func (*IntegrationTestContext) GetLogger() logr.Logger {
	return logr.DiscardLogger{}
}

var boolTrue = true

// AfterEach should be invoked by ginko.AfterEach to stop the guest cluster's
// API server.
func (ctx *IntegrationTestContext) AfterEach() {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ctx.Namespace,
		},
	}
	By("Destroying integration test namespace")
	Expect(ctx.Client.Delete(ctx, namespace)).To(Succeed())

	if ctx.envTest != nil {
		By("Shutting down guest cluster control plane")
		Expect(ctx.envTest.Stop()).To(Succeed())
	}
}

// NewIntegrationTestContext should be invoked by ginkgo.BeforeEach
//
// This function creates a VSphereCluster with a generated name, but stops
// short of generating a CAPI cluster so that it will work when the VSphere Cluster
// controller is also deployed.
//
// This function returns a TestSuite context
// The resources created by this function may be cleaned up by calling AfterEach
// with the IntegrationTestContext returned by this function.
func (s *TestSuite) NewIntegrationTestContext(goctx context.Context, integrationTestClient client.Client) *IntegrationTestContext {
	ctx := &IntegrationTestContext{
		Context: goctx,
		Client:  s.integrationTestClient,
		suite:   s,
	}

	By("Creating a temporary namespace", func() {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: uuid.New().String(),
			},
		}
		Expect(ctx.Client.Create(s, namespace)).To(Succeed())

		ctx.Namespace = namespace.Name
	})

	By("Create a vsphere cluster and wait for it to exist", func() {
		ctx.VSphereCluster = &vmwarev1.VSphereCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ctx.Namespace,
				Name:      "test-pvcsi",
			},
		}
		Expect(ctx.Client.Create(s, ctx.VSphereCluster)).To(Succeed())
		ctx.VSphereClusterKey = client.ObjectKey{Namespace: ctx.VSphereCluster.Namespace, Name: ctx.VSphereCluster.Name}
		Eventually(func() error {
			return ctx.Client.Get(s, ctx.VSphereClusterKey, ctx.VSphereCluster)
		}).Should(Succeed())

		ph, err := patch.NewHelper(ctx.VSphereCluster, ctx.Client)
		Expect(err).To(BeNil())
		ctx.PatchHelper = ph
	})

	By("Creating a extensions ca", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ctx.VSphereCluster.Name + "-extensions-ca",
				Namespace: ctx.Namespace,
			},
			Data: map[string][]byte{
				"ca.crt":  []byte("test-ca"),
				"tls.crt": []byte("test-tls.crt"),
				"tls.key": []byte("test-tls.key"),
			},
			Type: corev1.SecretTypeTLS,
		}
		Expect(ctx.Client.Create(s, secret)).To(Succeed())
		secretKey := client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}
		Eventually(func() error {
			return ctx.Client.Get(s, secretKey, secret)
		}).Should(Succeed())
	})
	return ctx
}

// NewIntegrationTestContextWithClusters should be invoked by ginkgo.BeforeEach.
//
// This function creates a VSphereCluster with a generated name as well as a
// CAPI Cluster with the same name. The function also creates a test environment
// and starts its API server to serve as the control plane endpoint for the
// guest cluster.
//
// This function returns a IntegrationTest context.
//
// The resources created by this function may be cleaned up by calling AfterEach
// with the IntegrationTestContext returned by this function.
func (s *TestSuite) NewIntegrationTestContextWithClusters(goctx context.Context, integrationTestClient client.Client, simulateControlPlane bool) *IntegrationTestContext {
	ctx := s.NewIntegrationTestContext(goctx, integrationTestClient)
	s.SetIntegrationTestClient(integrationTestClient)
	By("Create the CAPI Cluster and wait for it to exist", func() {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ctx.VSphereCluster.Namespace,
				Name:      ctx.VSphereCluster.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         ctx.VSphereCluster.APIVersion,
						Kind:               ctx.VSphereCluster.Kind,
						Name:               ctx.VSphereCluster.Name,
						UID:                ctx.VSphereCluster.UID,
						BlockOwnerDeletion: &boolTrue,
						Controller:         &boolTrue,
					},
				},
			},
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					Pods: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"1.0.0.0/16"},
					},
					Services: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"2.0.0.0/16"},
					},
				},
				InfrastructureRef: &corev1.ObjectReference{
					Name:      ctx.VSphereCluster.Name,
					Namespace: ctx.VSphereCluster.Namespace,
				},
			},
		}
		Expect(ctx.Client.Create(s, cluster)).To(Succeed())
		clusterKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}
		Eventually(func() error {
			return ctx.Client.Get(s, clusterKey, cluster)
		}).Should(Succeed())

		ctx.Cluster = cluster
		ctx.ClusterKey = clusterKey
	})

	if simulateControlPlane {
		var config *rest.Config
		By("Creating guest cluster control plane", func() {
			// Initialize a test environment to simulate the control plane of the
			// guest cluster.
			var err error
			ctx.envTest = &envtest.Environment{
				//KubeAPIServerFlags: append([]string{"--allow-privileged=true"}, envtest.DefaultKubeAPIServerFlags...),
				// Add some form of CRD so the CRD object is registered in the
				// scheme...
				CRDDirectoryPaths: []string{
					filepath.Join(s.flags.RootDir, "config", "default", "crd"),
					filepath.Join(s.flags.RootDir, "config", "supervisor", "crd"),
				},
			}
			ctx.envTest.ControlPlane.GetAPIServer().Configure().Set("allow-privileged", "true")
			config, err = ctx.envTest.Start()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(config).ShouldNot(BeNil())

			ctx.GuestClient, err = client.New(config, client.Options{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ctx.GuestClient).ShouldNot(BeNil())
		})

		By("Create the kubeconfig secret for the cluster", func() {
			buf, err := WriteKubeConfig(config)
			Expect(err).ToNot(HaveOccurred())
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ctx.Cluster.Namespace,
					Name:      fmt.Sprintf("%s-kubeconfig", ctx.Cluster.Name),
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: ctx.Cluster.APIVersion,
							Kind:       ctx.Cluster.Kind,
							Name:       ctx.Cluster.Name,
							UID:        ctx.Cluster.UID,
						},
					},
				},
				Data: map[string][]byte{
					"value": buf,
				},
			}
			Expect(ctx.Client.Create(s, secret)).To(Succeed())
			secretKey := client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}
			Eventually(func() error {
				return ctx.Client.Get(s, secretKey, secret)
			}).Should(Succeed())
		})
	}

	return ctx
}

// WriteKubeConfig writes an existing *rest.Config out as the typical
// KubeConfig YAML data.
func WriteKubeConfig(config *rest.Config) ([]byte, error) {
	return clientcmd.Write(api.Config{
		Clusters: map[string]*api.Cluster{
			config.ServerName: {
				Server:                   config.Host,
				CertificateAuthorityData: config.CAData,
			},
		},
		Contexts: map[string]*api.Context{
			config.ServerName: {
				Cluster:  config.ServerName,
				AuthInfo: config.Username,
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			config.Username: {
				ClientKeyData:         config.KeyData,
				ClientCertificateData: config.CertData,
			},
		},
		CurrentContext: config.ServerName,
	})
}
