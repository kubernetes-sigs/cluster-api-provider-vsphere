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

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

const (
	DefaultUserName           = "capv"
	VSpherePrivateKeyFilePath = "VSPHERE_SSH_PRIVATE_KEY"
)

type LogCollector struct{}

func (collector LogCollector) CollectMachinePoolLog(ctx context.Context, managementClusterClient client.Client, m *expv1.MachinePool, outputPath string) error {
	return nil
}

func (collector LogCollector) CollectMachineLog(_ context.Context, _ client.Client, m *clusterv1.Machine, outputPath string) error {
	var hostIPAddr string
	for _, address := range m.Status.Addresses {
		if address.Type != clusterv1.MachineExternalIP {
			continue
		}
		hostIPAddr = address.Address
		break
	}

	captureLogs := func(hostFileName, command string, args ...string) func() error {
		return func() error {
			f, err := createOutputFile(filepath.Join(outputPath, hostFileName))
			if err != nil {
				return err
			}
			defer f.Close()
			return executeRemoteCommand(f, hostIPAddr, command, args...)
		}
	}

	return kinderrors.AggregateConcurrent([]func() error{
		captureLogs("kubelet.log",
			"sudo journalctl", "--no-pager", "--output=short-precise", "-u", "kubelet.service"),
		captureLogs("containerd.log",
			"sudo journalctl", "--no-pager", "--output=short-precise", "-u", "containerd.service"),
		captureLogs("cloud-init.log",
			"cat", "/var/log/cloud-init.log"),
		captureLogs("cloud-init-output.log",
			"cat", "/var/log/cloud-init-output.log"),
	})
}

func createOutputFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(path)
}

func executeRemoteCommand(f io.StringWriter, hostIPAddr, command string, args ...string) error {
	config, err := newSSHConfig()
	if err != nil {
		return err
	}
	port := "22"

	hostClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", hostIPAddr, port), config)
	if err != nil {
		return errors.Wrapf(err, "dialing host IP address at %s", hostIPAddr)
	}
	defer hostClient.Close()

	session, err := hostClient.NewSession()
	if err != nil {
		return errors.Wrap(err, "opening SSH session")
	}
	defer session.Close()

	// Run the command and write the captured stdout to the file
	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}
	if err = session.Run(command); err != nil {
		return errors.Wrapf(err, "running command \"%s\"", command)
	}
	if _, err = f.WriteString(stdoutBuf.String()); err != nil {
		return errors.Wrap(err, "writing output to file")
	}

	return nil
}

// newSSHConfig returns a configuration to use for SSH connections to remote machines.
func newSSHConfig() (*ssh.ClientConfig, error) {
	sshPrivateKeyContent, err := readPrivateKey()
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(sshPrivateKeyContent)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("error parsing private key: %s", sshPrivateKeyContent))
	}

	config := &ssh.ClientConfig{
		User:            DefaultUserName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	return config, nil
}

func readPrivateKey() ([]byte, error) {
	privateKeyFilePath := os.Getenv(VSpherePrivateKeyFilePath)
	if len(privateKeyFilePath) == 0 {
		return nil, errors.Errorf("private key information missing. Please set %s environment variable", VSpherePrivateKeyFilePath)
	}

	return os.ReadFile(privateKeyFilePath)
}
