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

package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
)

const (
	rsaKeySize = 2048
)

// Reconcile generates an SSH key pair if one does not exist.
func Reconcile(ctx *context.ClusterContext) error {
	if ctx.ClusterConfig.SSHKeyPair.HasCertAndKey() {
		return nil
	}

	ctx.Logger.V(6).Info("reconciling SSH")

	private, public, err := NewKeyPair()
	if err != nil {
		return errors.Wrap(err, "error reconciciling SSH")
	}

	ctx.ClusterConfig.SSHKeyPair.Key = private
	ctx.ClusterConfig.SSHKeyPair.Cert = public
	return nil
}

// NewKeyPair generates a new SSH key pair.
func NewKeyPair() ([]byte, []byte, error) {
	// Generate the SSH private key.
	privKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return nil, nil, errors.Wrap(err, "rsa.GenerateKey failed")
	}
	if err := privKey.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "privKey.Validate failed")
	}
	privDER := x509.MarshalPKCS1PrivateKey(privKey)
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}
	privKeyBuf := pem.EncodeToMemory(&privBlock)

	// Generate the SSH public key.
	pubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "ssh.NewPublicKey failed")
	}
	pubKeyBuf := ssh.MarshalAuthorizedKey(pubKey)

	return privKeyBuf, pubKeyBuf, nil
}
