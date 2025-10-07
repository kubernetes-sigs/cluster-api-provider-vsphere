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

package kubernetes

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GenerateFiles generates control plane files for the current pod.
// The implementation assumes this code to be run as init container in the control plane pod; also
// it assume that secrets with cluster certificate authorities are mirrored in the backing cluster.
// Note: we are using the manager instead of another binary for convenience (the manager is already built and packaged
// into an image that is published during the release process).
func GenerateFiles(ctx context.Context, client client.Client) error {
	log := ctrl.LoggerFrom(ctx)

	// Gets the info about current pod.
	podNamespace := os.Getenv("POD_NAMESPACE")
	podName := os.Getenv("POD_NAME")
	podIP := os.Getenv("POD_IP")

	// Gets some additional info about the cluster.
	clusterName := os.Getenv("CLUSTER_NAME")
	controlPlaneEndpointHost := os.Getenv("CONTROL_PLANE_ENDPOINT_HOST")
	clusterKey := types.NamespacedName{Namespace: podNamespace, Name: clusterName}

	log.Info("Generating files", "POD_NAME", podName, "POD_NAMESPACE", podNamespace, "POD_IP", podIP, "CLUSTER_NAME", clusterName, "CONTROL_PLANE_ENDPOINT_HOST", controlPlaneEndpointHost)
	log.Info("Generating ca, apiserver, apiserver-kubelet-client certificate files")

	ca, err := getKeyCertPair(ctx, client, clusterKey, secret.ClusterCA)
	if err != nil {
		return err
	}

	if err := ca.WriteCertAndKey("/etc/kubernetes/pki", "ca"); err != nil {
		return err
	}

	if err := ca.WriteNewCertAndKey(apiServerCertificateConfig(podName, podIP, controlPlaneEndpointHost), "/etc/kubernetes/pki", "apiserver"); err != nil {
		return errors.Wrap(err, "failed to create API server")
	}

	if err := ca.WriteNewCertAndKey(apiServerKubeletClientCertificateConfig(), "/etc/kubernetes/pki", "apiserver-kubelet-client"); err != nil {
		return errors.Wrap(err, "failed to create API server kubelet client certificate")
	}

	log.Info("Generating front-proxy-ca, front-proxy-client certificate files")

	frontProxyCA, err := getKeyCertPair(ctx, client, clusterKey, secret.FrontProxyCA)
	if err != nil {
		return err
	}

	if err := frontProxyCA.WriteCertAndKey("/etc/kubernetes/pki", "front-proxy-ca"); err != nil {
		return err
	}

	if err := frontProxyCA.WriteNewCertAndKey(frontProxyClientCertificateConfig(), "/etc/kubernetes/pki", "front-proxy-client"); err != nil {
		return errors.Wrap(err, "failed to create front proxy client certificate")
	}

	log.Info("Generating sa key files")

	serviceAccountPrivateKey, serviceAccountPublicKey, err := getPrivatePublicKeyPair(ctx, client, clusterKey, secret.ServiceAccount)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join("/etc/kubernetes/pki", "sa.key"), serviceAccountPrivateKey, os.FileMode(0600)); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join("/etc/kubernetes/pki", "sa.pub"), serviceAccountPublicKey, os.FileMode(0600)); err != nil {
		return err
	}

	log.Info("Generating etcd ca, server, peer, apiserver-etcd-client certificate files")

	etcd, err := getKeyCertPair(ctx, client, clusterKey, secret.EtcdCA)
	if err != nil {
		return err
	}

	if err := etcd.WriteCertAndKey("/etc/kubernetes/pki/etcd", "ca"); err != nil {
		return err
	}

	if err := etcd.WriteNewCertAndKey(etcdServerCertificateConfig(podName, podIP), "/etc/kubernetes/pki/etcd", "server"); err != nil {
		return errors.Wrap(err, "failed to create etcd server certificate")
	}

	if err := etcd.WriteNewCertAndKey(etcdPeerCertificateConfig(podName, podIP), "/etc/kubernetes/pki/etcd", "peer"); err != nil {
		return errors.Wrap(err, "failed to create etcd peer certificate")
	}

	if err := etcd.WriteNewCertAndKey(apiServerEtcdClientCertificateConfig(), "/etc/kubernetes/pki", "apiserver-etcd-client"); err != nil {
		return errors.Wrap(err, "failed to create API server etcd client certificate")
	}

	log.Info("Generating admin, scheduler, controller-manager kubeconfig files")

	schedulerClient, err := ca.NewCertAndKey(schedulerClientCertificateConfig())
	if err != nil {
		return errors.Wrap(err, "failed to create scheduler client certificate")
	}

	schedulerKubeConfig := api.Config{
		Clusters: map[string]*api.Cluster{
			clusterKey.Name: {
				Server:                   "https://127.0.0.1:6443",
				CertificateAuthorityData: certs.EncodeCertPEM(ca.cert),
			},
		},
		Contexts: map[string]*api.Context{
			clusterKey.Name: {
				Cluster:  clusterKey.Name,
				AuthInfo: "scheduler",
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"scheduler": {
				ClientKeyData:         certs.EncodePrivateKeyPEM(schedulerClient.key),
				ClientCertificateData: certs.EncodeCertPEM(schedulerClient.cert),
			},
		},
		CurrentContext: clusterKey.Name,
	}
	if err := clientcmd.WriteToFile(schedulerKubeConfig, "/etc/kubernetes/scheduler.conf"); err != nil {
		return errors.Wrap(err, "failed to serialize scheduler kubeconfig")
	}

	controllerManagerClient, err := ca.NewCertAndKey(controllerManagerClientCertificateConfig())
	if err != nil {
		return errors.Wrap(err, "failed to create controller manager client certificate")
	}

	controllerManagerKubeConfig := api.Config{
		Clusters: map[string]*api.Cluster{
			clusterKey.Name: {
				Server:                   "https://127.0.0.1:6443",
				CertificateAuthorityData: certs.EncodeCertPEM(ca.cert),
			},
		},
		Contexts: map[string]*api.Context{
			clusterKey.Name: {
				Cluster:  clusterKey.Name,
				AuthInfo: "controller-manager",
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"controller-manager": {
				ClientKeyData:         certs.EncodePrivateKeyPEM(controllerManagerClient.key),
				ClientCertificateData: certs.EncodeCertPEM(controllerManagerClient.cert),
			},
		},
		CurrentContext: clusterKey.Name,
	}
	if err := clientcmd.WriteToFile(controllerManagerKubeConfig, "/etc/kubernetes/controller-manager.conf"); err != nil {
		return errors.Wrap(err, "failed to serialize scheduler kubeconfig")
	}

	adminClient, err := ca.NewCertAndKey(adminClientCertificateConfig())
	if err != nil {
		return errors.Wrap(err, "failed to create admin client certificate")
	}

	adminKubeConfig := api.Config{
		Clusters: map[string]*api.Cluster{
			clusterKey.Name: {
				Server:                   "https://127.0.0.1:6443",
				CertificateAuthorityData: certs.EncodeCertPEM(ca.cert),
			},
		},
		Contexts: map[string]*api.Context{
			clusterKey.Name: {
				Cluster:  clusterKey.Name,
				AuthInfo: "admin",
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"controller-manager": {
				ClientKeyData:         certs.EncodePrivateKeyPEM(adminClient.key),
				ClientCertificateData: certs.EncodeCertPEM(adminClient.cert),
			},
		},
		CurrentContext: clusterKey.Name,
	}
	if err := clientcmd.WriteToFile(adminKubeConfig, "/etc/kubernetes/admin.conf"); err != nil {
		return errors.Wrap(err, "failed to serialize admin kubeconfig")
	}

	log.Info("All file generated!")
	return nil
}

type KeyCertPair struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
}

// NewCertAndKey creates new certificate and key by passing the certificate authority certificate and key.
func (kp *KeyCertPair) NewCertAndKey(config *certs.Config) (*KeyCertPair, error) {
	key, err := certs.NewPrivateKey()
	if err != nil {
		return nil, errors.Wrap(err, "unable to create private key")
	}

	cert, err := config.NewSignedCert(key, kp.cert, kp.key)
	if err != nil {
		return nil, errors.Wrap(err, "unable to sign certificate")
	}

	return &KeyCertPair{
		key:  key,
		cert: cert,
	}, nil
}

func (kp *KeyCertPair) WriteCertAndKey(path, name string) error {
	if err := os.MkdirAll(path, os.FileMode(0755)); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(path, fmt.Sprintf("%s.key", name)), certs.EncodePrivateKeyPEM(kp.key), os.FileMode(0600)); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(path, fmt.Sprintf("%s.crt", name)), certs.EncodeCertPEM(kp.cert), os.FileMode(0600)); err != nil {
		return err
	}
	return nil
}

func (kp *KeyCertPair) WriteNewCertAndKey(config *certs.Config, path, name string) error {
	newKP, err := kp.NewCertAndKey(config)
	if err != nil {
		return err
	}
	return newKP.WriteCertAndKey(path, name)
}

func getKeyCertPair(ctx context.Context, client client.Client, cluster types.NamespacedName, purpose secret.Purpose) (*KeyCertPair, error) {
	certificates := secret.NewCertificatesForInitialControlPlane(nil)
	if err := certificates.Lookup(ctx, client, cluster); err != nil {
		return nil, errors.Wrap(err, "failed to lookup certificate secrets")
	}

	certificate := certificates.GetByPurpose(purpose)
	if certificate == nil {
		return nil, errors.Errorf("failed to lookup %s secret", purpose)
	}

	signer, err := certs.DecodePrivateKeyPEM(certificate.KeyPair.Key)
	if err != nil {
		return nil, errors.Errorf("failed to decode key from %s secret", purpose)
	}
	key, ko := signer.(*rsa.PrivateKey)
	if !ko {
		return nil, errors.Errorf("failed key from %s secret is not a valid rsa.PrivateKey", purpose)
	}

	cert, err := certs.DecodeCertPEM(certificate.KeyPair.Cert)
	if err != nil {
		return nil, errors.Errorf("failed to lookup key from %s secret", purpose)
	}

	return &KeyCertPair{
		key:  key,
		cert: cert,
	}, nil
}

func getPrivatePublicKeyPair(ctx context.Context, client client.Client, cluster types.NamespacedName, purpose secret.Purpose) (privateKey []byte, publicKey []byte, _ error) {
	certificates := secret.NewCertificatesForInitialControlPlane(nil)
	if err := certificates.Lookup(ctx, client, cluster); err != nil {
		return nil, nil, errors.Wrap(err, "failed to lookup certificate secrets")
	}

	certificate := certificates.GetByPurpose(purpose)
	if certificate == nil {
		return nil, nil, errors.Errorf("failed to lookup %s secret", purpose)
	}

	return certificate.KeyPair.Key, certificate.KeyPair.Cert, nil
}

func apiServerCertificateConfig(podName, podIP, controlPlaneEndpointHost string) *certs.Config {
	// create AltNames.DNSNames with defaults DNSNames.
	altNames := &certs.AltNames{
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			fmt.Sprintf("kubernetes.default.svc.%s", dnsDomain),
			"localhost",
			podName,
		},
		IPs: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
			net.ParseIP(podIP),
			// Note: we assume this is always an in IP (the cluster service IP)
			net.ParseIP(controlPlaneEndpointHost),
		},
	}

	return &certs.Config{
		CommonName: "apiserver",
		AltNames:   *altNames,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
}

func schedulerClientCertificateConfig() *certs.Config {
	return &certs.Config{
		CommonName:   "system:kube-scheduler",
		Organization: []string{},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func controllerManagerClientCertificateConfig() *certs.Config {
	return &certs.Config{
		CommonName:   "system:kube-controller-manager",
		Organization: []string{},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func adminClientCertificateConfig() *certs.Config {
	return &certs.Config{
		CommonName:   "admin",
		Organization: []string{"system:masters"},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func apiServerEtcdClientCertificateConfig() *certs.Config {
	return &certs.Config{
		CommonName:   "apiserver-etcd-client",
		Organization: []string{"system:masters"}, // TODO: check if we can drop
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func apiServerKubeletClientCertificateConfig() *certs.Config {
	return &certs.Config{
		CommonName:   "apiserver-kubelet-client",
		Organization: []string{"system:masters"},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func frontProxyClientCertificateConfig() *certs.Config {
	return &certs.Config{
		CommonName: "front-proxy-client",
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func etcdServerCertificateConfig(podName, podIP string) *certs.Config {
	// create AltNames with defaults DNSNames, IPs.
	altNames := certs.AltNames{
		DNSNames: []string{
			"localhost",
			podName,
		},
		IPs: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
			net.ParseIP(podIP),
		},
	}

	return &certs.Config{
		CommonName: podName,
		AltNames:   altNames,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
}

func etcdPeerCertificateConfig(podName, podIP string) *certs.Config {
	// create AltNames with defaults DNSNames, IPs.
	altNames := certs.AltNames{
		DNSNames: []string{
			"localhost",
			podName,
		},
		IPs: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
			net.ParseIP(podIP),
		},
	}

	return &certs.Config{
		CommonName: podName,
		AltNames:   altNames,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
}
