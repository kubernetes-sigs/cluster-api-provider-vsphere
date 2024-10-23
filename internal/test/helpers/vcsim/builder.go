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

package vcsim

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/vapi/cluster/simulator" // import this to register cluster module service test endpoint
	"sigs.k8s.io/cluster-api/util/certs"
)

// Builder helps in creating a vcsim simulator.
type Builder struct {
	skipModelCreate bool
	url             *url.URL
	model           *simulator.Model
	operations      []string
}

// NewBuilder returns a new a Builder.
func NewBuilder() *Builder {
	return &Builder{model: simulator.VPX()}
}

// WithModel defines the model to be used by the Builder when creating the vcsim instance.
func (b *Builder) WithModel(model *simulator.Model) *Builder {
	b.model = model
	return b
}

// SkipModelCreate tells the builder to skip creating the model, because it is already created before passing it
// to WithModel.
func (b *Builder) SkipModelCreate() *Builder {
	b.skipModelCreate = true
	return b
}

// WithURL defines the url to be used for service listening.
func (b *Builder) WithURL(url *url.URL) *Builder {
	b.url = url
	return b
}

// WithOperations defines the operation that the Builder should executed on the newly created vcsim instance.
func (b *Builder) WithOperations(ops ...string) *Builder {
	b.operations = append(b.operations, ops...)
	return b
}

// Build the vcsim instance.
func (b *Builder) Build() (*Simulator, error) {
	if !b.skipModelCreate {
		err := b.model.Create()
		if err != nil {
			return nil, err
		}
	}

	if b.url != nil {
		b.model.Service.Listen = b.url
	}

	b.model.Service.TLS = new(tls.Config)
	keyPair, err := generateTLSKeyPair()
	if err != nil {
		return nil, err
	}

	b.model.Service.TLS.Certificates = append(b.model.Service.TLS.Certificates, keyPair)
	b.model.Service.RegisterEndpoints = true
	server := b.model.Service.NewServer()
	simr := &Simulator{
		model:  b.model,
		server: server,
	}

	serverURL := server.URL
	pwd, _ := serverURL.User.Password()

	govcURL := fmt.Sprintf("https://%s:%s@%s", serverURL.User.Username(), pwd, serverURL.Host)
	for _, op := range b.operations {
		cmd := govcCommand(govcURL, op)
		if _, err := cmd.Output(); err != nil {
			simr.Destroy()
			return nil, err
		}
	}

	return simr, nil
}

func govcCommand(govcURL, commandStr string, buffers ...*gbytes.Buffer) *exec.Cmd {
	govcBinPath := os.Getenv("GOVC_BIN_PATH")
	args := strings.Split(commandStr, " ")
	cmd := exec.Command(govcBinPath, args...) //nolint:gosec // Non-production code
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOVC_URL=%s", govcURL), "GOVC_INSECURE=true")

	// the 1st arg for the buffer variadic input is set to stdout and the next one is set to stderr
	if len(buffers) > 0 {
		cmd.Stdout = buffers[0]
	}
	if len(buffers) > 1 {
		cmd.Stderr = buffers[1]
	}
	return cmd
}

// generateTLSKeyPair generates a self-signed certificatge keypair similar to the
// hardcoded values in github.com/vmware/govmomi/simulator/internal but adds the
// POD_IP to the IP Addresses of the certificate.
func generateTLSKeyPair() (tls.Certificate, error) {
	privateKey, err := certs.NewPrivateKey()
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			Organization: []string{"CAPV vcsim"},
		},
		NotBefore: time.Now().Add(time.Minute * -5),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365),

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
	}

	if ip := os.Getenv("POD_IP"); ip != "" {
		template.IPAddresses = append(template.IPAddresses, net.ParseIP(ip))
	}

	b, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "failed to create certificate")
	}

	cert, err := x509.ParseCertificate(b)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "failed to parse certificate")
	}

	return tls.X509KeyPair(certs.EncodeCertPEM(cert), certs.EncodePrivateKeyPEM(privateKey))
}
