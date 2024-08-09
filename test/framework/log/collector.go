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

// Package log provides utils for collecting logs from VMs.
package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	"golang.org/x/crypto/ssh"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/vmoperator"
)

const (
	DefaultUserName           = "capv"
	VSpherePrivateKeyFilePath = "VSPHERE_SSH_PRIVATE_KEY"
)

type MachineLogCollector struct {
	Client *govmomi.Client
	Finder *find.Finder
}

func (c *MachineLogCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *expv1.MachinePool, _ string) error {
	return nil
}

func (c *MachineLogCollector) CollectMachineLog(ctx context.Context, ctrlClient client.Client, m *clusterv1.Machine, outputPath string) error {
	machineIPAddresses, err := c.machineIPAddresses(ctx, ctrlClient, m)
	if err != nil {
		return err
	}

	captureLogs := func(hostFileName, command string, args ...string) func() error {
		return func() error {
			f, err := createOutputFile(filepath.Join(outputPath, hostFileName))
			if err != nil {
				return err
			}
			defer f.Close()
			var errs []error
			// Try with all available IPs unless it succeeded.
			for _, machineIPAddress := range machineIPAddresses {
				if err := executeRemoteCommand(f, machineIPAddress, command, args...); err != nil {
					errs = append(errs, err)
					continue
				}
				return nil
			}

			if err := kerrors.NewAggregate(errs); err != nil {
				return errors.Wrapf(err, "failed to run command %s for machine %s on ips [%s]", command, klog.KObj(m), strings.Join(machineIPAddresses, ", "))
			}
			return nil
		}
	}

	return aggregateConcurrent(
		captureLogs("kubelet.log",
			"sudo journalctl", "--no-pager", "--output=short-precise", "-u", "kubelet.service"),
		captureLogs("containerd.log",
			"sudo journalctl", "--no-pager", "--output=short-precise", "-u", "containerd.service"),
		captureLogs("cloud-init.log",
			"sudo", "cat", "/var/log/cloud-init.log"),
		captureLogs("cloud-init-output.log",
			"sudo", "cat", "/var/log/cloud-init-output.log"),
		captureLogs("kubeadm-service.log",
			"sudo", "cat", "/var/log/kubeadm-service.log"),
	)
}

func (c *MachineLogCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return nil
}

func (c *MachineLogCollector) machineIPAddresses(ctx context.Context, ctrlClient client.Client, m *clusterv1.Machine) ([]string, error) {
	for _, address := range m.Status.Addresses {
		if address.Type != clusterv1.MachineExternalIP {
			continue
		}
		return []string{address.Address}, nil
	}

	vmName := m.GetName()

	// For supervisor mode it could be the case that the name of the virtual machine differs from the machine's name.
	if m.Spec.InfrastructureRef.GroupVersionKind().Group == vmwarev1.GroupVersion.Group {
		vsphereMachine := &vmwarev1.VSphereMachine{}
		if err := ctrlClient.Get(ctx, client.ObjectKey{Namespace: m.Spec.InfrastructureRef.Namespace, Name: m.Spec.InfrastructureRef.Name}, vsphereMachine); err != nil {
			return nil, errors.Wrapf(err, "getting vmwarev1.VSphereMachine %s/%s", m.Spec.InfrastructureRef.Namespace, m.Spec.InfrastructureRef.Name)
		}

		if vsphereMachine.Status.IPAddr != "" {
			return []string{vsphereMachine.Status.IPAddr}, nil
		}

		var err error
		vmName, err = vmoperator.GenerateVirtualMachineName(m.Name, vsphereMachine.Spec.NamingStrategy)
		if err != nil {
			return nil, errors.Wrapf(err, "generating VirtualMachine name for Machine %s/%s", m.Namespace, m.Name)
		}
	}

	vmObj, err := c.Finder.VirtualMachine(ctx, vmName)
	if err != nil {
		return nil, err
	}

	var vm mo.VirtualMachine

	if err := c.Client.RetrieveOne(ctx, vmObj.Reference(), []string{"guest.net"}, &vm); err != nil {
		// We cannot get the properties e.g. when the vm already got deleted or is getting deleted.
		return nil, errors.Errorf("error retrieving properties for machine %s", klog.KObj(m))
	}

	addresses := []string{}

	// Return all IPs so we can try each of them until one succeeded.
	for _, nic := range vm.Guest.Net {
		if nic.IpConfig == nil {
			continue
		}
		for _, ip := range nic.IpConfig.IpAddress {
			netIP := net.ParseIP(ip.IpAddress)
			ipv4 := netIP.To4()
			if ipv4 != nil {
				addresses = append(addresses, ip.IpAddress)
			}
		}
	}

	if len(addresses) == 0 {
		return nil, errors.Errorf("unable to find IP Addresses for Machine %s", klog.KObj(m))
	}

	return addresses, nil
}

func createOutputFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	return os.Create(filepath.Clean(path))
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // Non-production code
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	return config, nil
}

func readPrivateKey() ([]byte, error) {
	privateKeyFilePath := os.Getenv(VSpherePrivateKeyFilePath)
	if privateKeyFilePath == "" {
		return nil, errors.Errorf("private key information missing. Please set %s environment variable", VSpherePrivateKeyFilePath)
	}

	return os.ReadFile(filepath.Clean(privateKeyFilePath))
}

// aggregateConcurrent runs fns concurrently, returning aggregated errors.
func aggregateConcurrent(funcs ...func() error) error {
	// run all fns concurrently
	ch := make(chan error, len(funcs))
	var wg sync.WaitGroup
	for _, f := range funcs {
		f := f
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- f()
		}()
	}
	wg.Wait()
	close(ch)
	// collect up and return errors
	errs := []error{}
	for err := range ch {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}
