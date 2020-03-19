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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"
)

const (
	// rsaKeySize is the size of the generated private keys.
	rsaKeySize = 2048

	// pemTypeCertificate is the type used when encoding public keys as PEM
	// data.
	pemTypeCertificate = "CERTIFICATE"
	// pemTypeRSAPrivateKey is the type used when encoding private keys as PEM
	// data.
	pemTypeRSAPrivateKey = "RSA PRIVATE KEY"
)

func generateSigningCertificateKeyPair(notBefore, notAfter time.Time) (publicKeyPEM []byte, privateKeyPEM []byte, _ error) {

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return nil, nil, err
	}
	caSerial := newSerial(notBefore)
	caTemplate := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			CommonName:   "self-signed CA",
			SerialNumber: caSerial.String(),
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		SubjectKeyId:          bigIntHash(caPrivateKey.N),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, err
	}
	ca, _ := x509.ParseCertificate(caBytes)

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  pemTypeCertificate,
		Bytes: ca.Raw,
	})
	caPrivateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  pemTypeRSAPrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	})

	return caCertPEM, caPrivateKeyPEM, nil
}

func generateAndSignClientCertificateKeyPair(
	signingCertificatePEM []byte,
	signingKeyPEM []byte,
	notBefore time.Time,
	notAfter time.Time,
	serverIPAddr string) (publicKeyPEM []byte, privateKeyPEM []byte, _ error) {

	signingCertificatePEMBlock, _ := pem.Decode(signingCertificatePEM)
	signingCertificate, err := x509.ParseCertificate(signingCertificatePEMBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	signingKeyPEMBlock, _ := pem.Decode(signingKeyPEM)
	signingKey, err := x509.ParsePKCS1PrivateKey(signingKeyPEMBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	certSerial := newSerial(notBefore)
	certPrivateKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return nil, nil, err
	}
	certTemplate := &x509.Certificate{
		SerialNumber: certSerial,
		Subject:      pkix.Name{CommonName: serverIPAddr},
		IPAddresses:  []net.IP{net.ParseIP(serverIPAddr)},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		SubjectKeyId: bigIntHash(certPrivateKey.N),
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageDataEncipherment |
			x509.KeyUsageKeyEncipherment |
			x509.KeyUsageContentCommitment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		certTemplate,
		signingCertificate,
		&certPrivateKey.PublicKey,
		signingKey)
	if err != nil {
		return nil, nil, err
	}
	cert, _ := x509.ParseCertificate(certBytes)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  pemTypeCertificate,
		Bytes: cert.Raw,
	})
	certPrivateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  pemTypeRSAPrivateKey,
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivateKey),
	})

	return certPEM, certPrivateKeyPEM, nil
}

func newSerial(now time.Time) *big.Int {
	return big.NewInt(int64(now.Nanosecond()))
}

func bigIntHash(n *big.Int) []byte {
	h := sha256.New()
	_, _ = h.Write(n.Bytes())
	return h.Sum(nil)
}
