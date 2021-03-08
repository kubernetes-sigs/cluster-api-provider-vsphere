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

package haproxy

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
)

const (
	// SecretSuffixCA is the suffix appended to the name of the
	// HAProxyLoadBalancer resource to generate the name of the Secret
	// resource for the signing certificate and key data.
	// nolint:gosec
	SecretSuffixCA = "-haproxy-ca"

	// SecretSuffixConfig is the suffix appended to the name of the
	// HAProxyLoadBalancer resource to generate the name of the Secret
	// resource for the HAProxy API server configuration.
	// nolint:gosec
	SecretSuffixConfig = "-haproxy-config"

	// SecretSuffixBootstrap is the suffix appended to the name of the
	// HAProxyLoadBalancer resource to generate the name of the Secret
	// resource for bootstrap data required to create a new VM.
	// nolint:gosec
	SecretSuffixBootstrap = "-haproxy-bootstrap"

	// SecretDataKey is the key used by the Secret resources for the HAProxy
	// API config and bootstrap data to store their respective information.
	SecretDataKey = "value"

	// SecretDataKeyCAKey is the key used by the Secret resource for the
	// signing certificate/key pair that references the PEM-encoded, private
	// key data.
	SecretDataKeyCAKey = "ca.key"

	// SecretDataKeyCACert is the key used by the Secret resource for the
	// signing certificate/key pair that references the PEM-encoded, public
	// key data.
	SecretDataKeyCACert = "ca.cert"

	// SecretDataKeyUsername is the key used by the Secret resource for the
	// signing certificate/key pair that references the username.
	SecretDataKeyUsername = "username"

	// SecretDataKeyPassword is the key used by the Secret resource for the
	// signing certificate/key pair that references the password.
	SecretDataKeyPassword = "password"

	// DefaultNegativeTimeSkew is the time by which a certificate's validity should be set in the past to
	// account for clock skew
	DefaultNegativeTimeSkew = -10 * time.Minute
)

// NameForCASecret returns the name of the Secret for the signing
// certificate/key pair used to create bootstrap data and sign new client
// certificates.
func NameForCASecret(loadBalancerName string) string {
	return nameForSecret(loadBalancerName, SecretSuffixCA)
}

// NameForBootstrapSecret returns the name of the Secret for the bootstrap data
// used to create a new load balancer VM.
func NameForBootstrapSecret(loadBalancerName string) string {
	return nameForSecret(loadBalancerName, SecretSuffixBootstrap)
}

// NameForConfigSecret returns the name of the Secret for the HAProxy API
// config used to access the HAProxy API server.
func NameForConfigSecret(loadBalancerName string) string {
	return nameForSecret(loadBalancerName, SecretSuffixConfig)
}

func nameForSecret(loadBalancerName, secretSuffix string) string {
	return loadBalancerName + secretSuffix
}

// GetCASecret returns the Secret for the signing certificate/key pair used to
// create bootstrap data and sign new client certificates.
func GetCASecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName string) (*corev1.Secret, error) {

	return getSecret(ctx, client, secretNamespace, loadBalancerName, SecretSuffixCA)
}

// GetBootstrapSecret returns the Secret for the bootstrap data used to create
// a new load balancer VM.
func GetBootstrapSecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName string) (*corev1.Secret, error) {

	return getSecret(ctx, client, secretNamespace, loadBalancerName, SecretSuffixBootstrap)
}

// GetConfigSecret returns the Secret for the HAProxy API config used to access
// the HAProxy API server.
func GetConfigSecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName string) (*corev1.Secret, error) {

	return getSecret(ctx, client, secretNamespace, loadBalancerName, SecretSuffixConfig)
}

func getSecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName, secretNameSuffix string) (*corev1.Secret, error) {

	key := ctrlclient.ObjectKey{
		Namespace: secretNamespace,
		Name:      nameForSecret(loadBalancerName, secretNameSuffix),
	}
	obj := &corev1.Secret{}
	if err := client.Get(ctx, key, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// DeleteCASecret deletes the Secret for the signing certificate/key pair used
// to create bootstrap data and sign new client certificates.
func DeleteCASecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName string) error {

	return deleteSecret(ctx, client, secretNamespace, loadBalancerName, SecretSuffixCA)
}

// DeleteBootstrapSecret deletes the Secret for the bootstrap data used to
// create a new load balancer VM.
func DeleteBootstrapSecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName string) error {

	return deleteSecret(ctx, client, secretNamespace, loadBalancerName, SecretSuffixBootstrap)
}

// DeleteConfigSecret deletes the Secret for the HAProxy API config used to
// access the HAProxy API server.
func DeleteConfigSecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName string) error {

	return deleteSecret(ctx, client, secretNamespace, loadBalancerName, SecretSuffixConfig)
}

func deleteSecret(
	ctx context.Context,
	client ctrlclient.Client,
	secretNamespace, loadBalancerName, secretNameSuffix string) error {

	obj, err := getSecret(ctx, client, secretNamespace, loadBalancerName, secretNameSuffix)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	if obj.DeletionTimestamp.IsZero() {
		if err := client.Delete(ctx, obj); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}
	}
	return nil
}

func newDefaultCertificateValidity() (time.Time, time.Time) {
	notBefore := time.Now().UTC().Add(DefaultNegativeTimeSkew)
	notAfter := time.Now().UTC().Add(DefaultNegativeTimeSkew).AddDate(10, 0, 0)
	return notBefore, notAfter
}

// CreateCASecret creates the Secret resource that contains the signing
// certificate and key used to generate bootstrap data and sign client
// certificates.
func CreateCASecret(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	loadBalancer *infrav1.HAProxyLoadBalancer) error {

	crt, key, err := generateSigningCertificateKeyPair(newDefaultCertificateValidity())
	if err != nil {
		return err
	}

	return client.Create(ctx, &corev1.Secret{
		ObjectMeta: objectMetaForSecret(cluster, loadBalancer, SecretSuffixCA),
		Data: map[string][]byte{
			SecretDataKeyCACert:   crt,
			SecretDataKeyCAKey:    key,
			SecretDataKeyUsername: []byte(uuid.NewUUID()),
			SecretDataKeyPassword: []byte(uuid.NewUUID()),
		},
	})
}

// CreateBootstrapSecret creates the Secret resource that contains
// the bootstrap data required to create the load balancer VM.
func CreateBootstrapSecret(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	loadBalancer *infrav1.HAProxyLoadBalancer) error {

	caSecret, err := GetCASecret(ctx, client, loadBalancer.Namespace, loadBalancer.Name)
	if err != nil {
		return err
	}

	renderConfig := NewRenderConfiguration().
		WithBootstrapInfo(
			*loadBalancer,
			string(caSecret.Data[SecretDataKeyUsername]),
			string(caSecret.Data[SecretDataKeyPassword]),
			caSecret.Data[SecretDataKeyCACert],
			caSecret.Data[SecretDataKeyCAKey],
		)

	bootstrapData, err := renderConfig.BootstrapDataForLoadBalancer()
	if err != nil {
		return err
	}

	return client.Create(ctx, &corev1.Secret{
		ObjectMeta: objectMetaForSecret(cluster, loadBalancer, SecretSuffixBootstrap),
		Data: map[string][]byte{
			SecretDataKey: bootstrapData,
		},
	})
}

// CreateConfigSecret creates the Secret resource that contains
// the config data required to access the HAProxy API server.
func CreateConfigSecret(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	loadBalancer *infrav1.HAProxyLoadBalancer) error {

	caSecret, err := GetCASecret(ctx, client, loadBalancer.Namespace, loadBalancer.Name)
	if err != nil {
		return err
	}

	notBefore, notAfter := newDefaultCertificateValidity()

	clientCertPEM, clientKeyPEM, err := generateAndSignClientCertificateKeyPair(
		caSecret.Data[SecretDataKeyCACert],
		caSecret.Data[SecretDataKeyCAKey],
		notBefore,
		notAfter,
		loadBalancer.Status.Address)
	if err != nil {
		return err
	}

	config := &DataplaneConfig{
		CertificateAuthorityData: caSecret.Data[SecretDataKeyCACert],
		ClientCertificateData:    clientCertPEM,
		ClientKeyData:            clientKeyPEM,
		Server:                   fmt.Sprintf("https://%s:5556/v1", loadBalancer.Status.Address),
		Username:                 string(caSecret.Data[SecretDataKeyUsername]),
		Password:                 string(caSecret.Data[SecretDataKeyPassword]),
	}

	configData, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return client.Create(ctx, &corev1.Secret{
		ObjectMeta: objectMetaForSecret(cluster, loadBalancer, SecretSuffixConfig),
		Data: map[string][]byte{
			SecretDataKey: configData,
		},
	})
}

func objectMetaForSecret(
	cluster *clusterv1.Cluster,
	loadBalancer *infrav1.HAProxyLoadBalancer,
	secretSuffix string) metav1.ObjectMeta {

	return metav1.ObjectMeta{
		Namespace: loadBalancer.Namespace,
		Name:      loadBalancer.Name + secretSuffix,
		Labels: map[string]string{
			clusterv1.ClusterLabelName: cluster.Name,
		},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: loadBalancer.APIVersion,
				Kind:       loadBalancer.Kind,
				Name:       loadBalancer.Name,
				UID:        loadBalancer.UID,
			},
		},
	}
}
