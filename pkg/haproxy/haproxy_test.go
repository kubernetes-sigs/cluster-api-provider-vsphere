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

package haproxy_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/antihax/optional"
	"github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	hapi "sigs.k8s.io/cluster-api-provider-vsphere/contrib/haproxy/openapi"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/haproxy"
)

const testHAPIConfigFormat = `debug: %v
server: https://localhost:%d/v1
certificateAuthorityData: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURqekNDQW5lZ0F3SUJBZ0lKQVB1WnI3bkwvdFJsTUEwR0NTcUdTSWIzRFFFQkJRVUFNR1V4Q3pBSkJnTlYKQkFZVEFsVlRNUk13RVFZRFZRUUlEQXBEWVd4cFptOXlibWxoTVJJd0VBWURWUVFIREFsUVlXeHZJRUZzZEc4eApEekFOQmdOVkJBb01CbFpOZDJGeVpURU5NQXNHQTFVRUN3d0VRMEZRVmpFTk1Bc0dBMVVFQXd3RVkyRndkakFlCkZ3MHhPVEV5TWpNeE9EUXpNemRhRncweU9URXlNakF4T0RRek16ZGFNR1V4Q3pBSkJnTlZCQVlUQWxWVE1STXcKRVFZRFZRUUlEQXBEWVd4cFptOXlibWxoTVJJd0VBWURWUVFIREFsUVlXeHZJRUZzZEc4eER6QU5CZ05WQkFvTQpCbFpOZDJGeVpURU5NQXNHQTFVRUN3d0VRMEZRVmpFTk1Bc0dBMVVFQXd3RVkyRndkakNDQVNJd0RRWUpLb1pJCmh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTmZOQ2VEdGd2UjVMdWtBbUlSN0ZTdyt2azlEOUVhZ0xvSnoKcC9QYkNzT3pCNEhLMmtQTVBhM2NvK1BSQVVQMGhaWnp5S2hoSzhGWkVVd204Wnk2YTdTSTlwR0N5emkySktvNgpmSXpXWEdScUtzaGt3SlNXRmtib0FNd0hRTnBMNzhibHBsTTRSUlVaSHNvWHZzbHdTdGtvaEIyL2IycWNLOStYCk4zempjY2ZmRUNCTm1RWHVWU1Q3ZG5JTllsMWM0VkRZVXdIUE13Vk5sZWVOSURYU1l1VXMyemxLcGNJalNlTHEKNUtLQWVkN2lldzc1R3MrdnZCYmplTXFWSk5GWlZCVUUvVlVvNUFoMkJMTXNucDBIbzhrWkdKSUFXK1FZTFk0Ywp3YnJZQnBuaklWRnYwN2VpYTYwT3doald2R2xvcElOZnVQSGhXVmtyZVVOa3l2RHVWNjBDQXdFQUFhTkNNRUF3CkR3WURWUjBUQVFIL0JBVXdBd0VCL3pBT0JnTlZIUThCQWY4RUJBTUNBWVl3SFFZRFZSME9CQllFRkE4UTAxNmUKM0pENU5aR3lRcVU1NHhEVjJ1UExNQTBHQ1NxR1NJYjNEUUVCQlFVQUE0SUJBUURNSHhMRHBmQkVRVHI4bXBDSQpNclNVN2xzb09DanJKcGxET3NjTTk0eGE4R1R3VzlRdzhuTEJXOGZUczdqeG9VVmZPcmZHS1hWSEkxSytjSDJkCjloZlpyY3BGYjdpcmp5TXB6c3QxNnRRZFBSMldCT2I4RkJhMk5lVWxwSzhJajNXc0p5ZFNEOHdBRDB1SWovbDIKTkNrd0xtSDRMTDA0ZmhaeEM1R2sraGFOZjZtWGhxbVg5L1M5RWRkbTFQN2dma0V2YVA4bVFSNklOSXBnMmFoTgpGTzdjNkdNRDg2YlpxcmNuZ2dUNG9uV3dEN3pZRlEyMXg1NDVYY3BvWUd5STRyUlJUbVVlWC9BTXJ4Nm0zb00yCmpBZmVscytSR01vbytXZ05UekZCWnp0b1k0WkZMSDhGZVFsV1BnRDRad0FULzNMN2dNbDZNOVFKMzByVlg1MkoKNGEyWQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
clientCertificateData: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURwVENDQW8yZ0F3SUJBZ0lKQU1YQ2ozRW1CeXVMTUEwR0NTcUdTSWIzRFFFQkJRVUFNR1V4Q3pBSkJnTlYKQkFZVEFsVlRNUk13RVFZRFZRUUlEQXBEWVd4cFptOXlibWxoTVJJd0VBWURWUVFIREFsUVlXeHZJRUZzZEc4eApEekFOQmdOVkJBb01CbFpOZDJGeVpURU5NQXNHQTFVRUN3d0VRMEZRVmpFTk1Bc0dBMVVFQXd3RVkyRndkakFlCkZ3MHhPVEV5TWpNeU1qQTBNRFphRncweU9URXlNakF5TWpBME1EWmFNR1V4Q3pBSkJnTlZCQVlUQWxWVE1STXcKRVFZRFZRUUlEQXBEWVd4cFptOXlibWxoTVJJd0VBWURWUVFIREFsUVlXeHZJRUZzZEc4eER6QU5CZ05WQkFvTQpCbFpOZDJGeVpURU5NQXNHQTFVRUN3d0VRMEZRVmpFTk1Bc0dBMVVFQXd3RVkyRndkakNDQVNJd0RRWUpLb1pJCmh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBT0lXZmU5TDduT01jQk5kbVdXdnR5VkIvbktyVnVqVlA5eU8KYWhzOWRHNWF2V3JTS3A2VFpxUXFIeVF1bnoyUDFqLzNlVU5CVnNCN3EyaWFjN2x2RWFzOGk4Z3ZNNmNuWmpDeQpzUHVuT3BjSFR0bVo3Y2tsVzJ4SmsvNjdnN0NkZXJNSFVGbEVLWjJHZzBkS3kxR0hUMEs2MHhCcGJiY3I3VUxICmJBc1NXZnBRcXIzSW9MSlZJejBWSWt5Wm9GRGM4TXE4eVdGeFRGcGRNNTN0SVE1dFJKYkZwWFZxMUZLUjBRbk0KZlNncGwzQlhIZjlocW9pRXd0dDBOTGFUYThvajMzZ2ZBZDlKZGtKeVFHTXhpblJGTkdJVGxTNjNJM3JkQXIyRQpiVFo2cFYxWG5HYk93ZlFqRE1QQjRlUjdMSDk3SUYrbnJwUHlwK0lkejB3aEdNckZBVFVDQXdFQUFhTllNRll3CkNRWURWUjBUQkFJd0FEQUxCZ05WSFE4RUJBTUNCYUF3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0cKQVFVRkJ3TUJNQjBHQTFVZERnUVdCQlJQZ3NIWU13a2tVdGJJM2VtTmxBOE0vR0o2VlRBTkJna3Foa2lHOXcwQgpBUVVGQUFPQ0FRRUFiZnlSRHRoWGlMRUNtQ0k5Y1FlNlE5d01TU3VwcXd1UmZZWmpNUGNXZktpcVRTbHp1ZzJ6Cks1aTBEYWtzWmN6a1NhYlpRWTRDMkRoYzRJWTJXdkRaRTZDRXJNbU12V2diQzY4VXkzZkppeXl4WVpzbEE3OVIKN3RCcU55alovdUQvM2hseEMrdGo2VzZLMDFnOHBabmZ0SkxxbTFQYm9iUFRPem40T09iUGZiOHJVcldVdk4rTgptSUNlcU56bDlOYU95bEtvNkt0cFpyZDZ3MCtBRUJoTjBPNy8zVkIyc211L2l3Q3Z1c1NBWDBrcWlLNXIwbTZmCk0xSDNrc0k2anp1SGJsNER6aGlPR3lVcFBLY3pIc0c5S1dpZDU4WjMvSldsODZKNGpFMXl0OHpkQVA3ZmsrZE8KNE9FVDE5cE1tTUhZZzlOS1JXMUhwUWhVYngzb09oRXYxZz09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
clientKeyData: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBNGhaOTcwdnVjNHh3RTEyWlphKzNKVUgrY3F0VzZOVS8zSTVxR3oxMGJscTlhdElxCm5wTm1wQ29mSkM2ZlBZL1dQL2Q1UTBGV3dIdXJhSnB6dVc4UnF6eUx5Qzh6cHlkbU1MS3crNmM2bHdkTzJabnQKeVNWYmJFbVQvcnVEc0oxNnN3ZFFXVVFwbllhRFIwckxVWWRQUXJyVEVHbHR0eXZ0UXNkc0N4SlorbENxdmNpZwpzbFVqUFJVaVRKbWdVTnp3eXJ6SllYRk1XbDB6bmUwaERtMUVsc1dsZFdyVVVwSFJDY3g5S0NtWGNGY2QvMkdxCmlJVEMyM1EwdHBOcnlpUGZlQjhCMzBsMlFuSkFZekdLZEVVMFloT1ZMcmNqZXQwQ3ZZUnRObnFsWFZlY1pzN0IKOUNNTXc4SGg1SHNzZjNzZ1g2ZXVrL0tuNGgzUFRDRVl5c1VCTlFJREFRQUJBb0lCQVFDTnRiZGQ1R1Fqdk9VSwozbUlsNEl1VktOWktIYWN0N1d4SDNHUVppdDJOeGdad0RDZDJtY0YrS0lDNGR4aU14N2x0QXJyWk12MGpUT0RWCmdlb0RVdURxU2RyN3NNcFpmVktLTjViRFJjQnRwY0VBbDRENTBSYUt1MXV1RU82c0p5a2ZTZmhNMjNLU01CdmMKOWI2VzdZNzZyb3RaQUJ3cThiZVhZZFFRNUlITmFORFR2TnZybXRPcFZuekhqd2N3MFpXTUxxd2lwY211aExvdAp2bTlScmlPY0lKcUFUMTNMQWtwNCsyOWZwUDVSTVRtTnNlVFlacnlOOVVLZzFCVG9lUDhqVkFMeGd4NDh5eExFCnRZVkhOM1RJcTVwTms3RDFDSi9IQWNNaHhFTDNENVN0MFJHQ3Y3R3pHV0M1K1drNDg2Vi90bmhwdktJSlBJKzUKblNoQnN1eEJBb0dCQVBYRFA5LzhxRFZPRnVod2RvOWErVDNyT0NOMmlYTmxBbmRKZ0NMZHY4S2YxdTkybzhodQptS0xHdm5TYm50Njd6VldJWldTU21zdENuOHpTWU5qOTZCa0NtSGlJWHJoQWVlQlFQSnNmNjBrTUJucllncy9aCmdPcXpIOUpPdzFOY25Ja3loWlZTaklubjM5cldGaUtUV0k3Q1J5ZVJkQzRJTXhRVS92UUx3K0pOQW9HQkFPdUIKYjEyVWNRdEY1RXpLNW54d2VBNTVJd0NmZjVWVGl3Um9IWDJGeVRVU2FobjlLNlM4UC9GTitWUU11NWZzYXVlawo2MGdISGZMdVhBWWlqUTFLbTRiZGdaeUpoVkxOK1d6TkhtREpHUXN1WFBSSitpZEhUeTBSMzlNQ1N4Y05Tb25RCitUaXNNeWczRUVJS1ZKMjRKNE9sZ1B1c0g3TlB2NnlobTg1dFMzNkpBb0dCQU9MYjZRcUozM3ZWS2JCR29DcVUKZjU1NGtzbXBraGZERmhPbTlYRTU0TmwzVXFDWmszWmhJT1NoTVEzUzJVUWhkOW1Nbm92SUNMdTROR3FOaUhqRgphSW90cXpFWU1OZEVMVHl5MUQ4ZHA4TTJKb1VmZHlFR1ZjcFFydjhqVllxTjRyR0N3V3lsVnJXMkpSMk1vY0lvCjRZWm1MK2lHakFneDZYU1FMUWg2RThmQkFvR0FiUVcvaTEvRHNVZEt0KzRhSXpOaHNMbU5aYVZ3eDYwa0p3Y1gKMTlzT1dWNUw5Zm9Jc1R0Z2twSFpRWHFmZ1dZMTIwU3lrdWFRaTd5aXAwaHBhZVRHK1Bra0hsWmZmUVRUV2ZYZgpBVWszS2NEdDBUMUo2OU1NS1Q0a0VxZjJJUmJMRWQvRzcrQnYwa2NqWko4cHF0WHNuUG9LS3ZmMHVPckxQZHlXCnAwcGJiNWtDZ1lCVHMrL1JicWRlVHBia0tYQmNhWVljSXByOStIT2lhR0xsMUxIaHVPUHpVOEFxVjVtT1hnNGQKamlkdEx3bE9rbHY2d1V2TVZKTWNoVWpKSkhwWTlrbjZFKyszUXZCMjlCY2ZLODVmUDlvS2YwV0IwTHAwejEzRwp0K2QzWStUQWNvNmx6Z28xbElVbi9MeHhQb2RlTGlDTjA0c2FMeTdqR2xVbVdRRjUzbFNOMlE9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
`

var flagRunCreateLoadBalancer = flag.Bool("hapi.createLoadBalancer", false, "activates test for creating a load balancer; requires docker")

func TestMain(m *testing.M) {
	exitCode := m.Run()
	if *flagRunCreateLoadBalancer {
		if err := runDocker("kill", "haproxy", "http1", "http2"); err != nil {
			fmt.Println("unable to initialize load balancer")
		}
	}
	os.Exit(exitCode)
}

func TestCreateLoadBalancer(t *testing.T) {
	if !*flagRunCreateLoadBalancer {
		t.Skip("specify -hapi.createLoadBalancer to run this test")
	}
	g := gomega.NewWithT(t)
	gomega.SetDefaultEventuallyTimeout(time.Second * 10)
	gomega.SetDefaultEventuallyPollingInterval(time.Second * 1)

	// Build the HAProxy LB image. This can be built from the root of this
	// project in the hack/tools/haproxy directory.
	g.Expect(runDocker("build", "-t", "haproxy", "../../hack/tools/haproxy")).To(
		gomega.Succeed(), "failed to build the haproxy load balancer image")

	// Get random ports for the HAProxy dataplane API server and the LB
	// endpoint.
	apiPort := int32(RandomTCPPort())
	lbPort := int32(RandomTCPPort())

	// Start the HAProxy load balaner image.
	g.Expect(runDocker(
		"run", "--name", "haproxy", "-d", "--rm",
		"-p", fmt.Sprintf("%d:5556", apiPort),
		"-p", fmt.Sprintf("%d:8085", lbPort),
		"haproxy")).To(
		gomega.Succeed(), "failed to start the haproxy load balancer container")

	ctx := context.Background()

	testHAPIConfig := fmt.Sprintf(testHAPIConfigFormat, testing.Verbose(), apiPort)
	config, err := haproxy.LoadDataplaneConfig([]byte(testHAPIConfig))
	g.Expect(err).ToNot(gomega.HaveOccurred(), "failed create HAPI config from bytes")
	client, err := haproxy.ClientFromHAPIConfig(config)
	g.Expect(err).ToNot(gomega.HaveOccurred(), "failed create HAPI client from config")

	// Get the current configuration version.
	var version int32
	g.Eventually(func() error {
		global, resp, err := client.GlobalApi.GetGlobal(ctx, nil)
		if err == nil {
			g.Expect(resp.Body.Close()).To(gomega.Succeed())
			version = global.Version
		}
		return err
	}).ShouldNot(gomega.HaveOccurred(), "failed to get global HAPI config")

	// Start a transaction.
	var nextVersion optional.Int32
	g.Eventually(func() error {
		txn, resp, err := client.TransactionsApi.StartTransaction(ctx, version)
		if err != nil {
			return err
		}
		g.Expect(resp.Body.Close()).To(gomega.Succeed())
		nextVersion = optional.NewInt32(txn.Version)
		return nil
	}).ShouldNot(gomega.HaveOccurred(), "failed to start a transaction")

	// Start two web servers.
	g.Expect(runDocker(
		"run", "--name", "http1", "-d", "--rm",
		"nginxdemos/hello:plain-text")).To(
		gomega.Succeed(), "failed to start first web server")
	g.Expect(runDocker(
		"run", "--name", "http2", "-d", "--rm",
		"nginxdemos/hello:plain-text")).To(
		gomega.Succeed(), "failed to start second web server")

	// Get the IP address of the first web server.
	stdout, _, err := runWithOpts("docker",
		runOptions{printStdout: true, printStderr: true},
		"inspect", "http1", "-f", "'{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'")
	g.Expect(err).ToNot(gomega.HaveOccurred(), "failed to get IP address of first web server")
	http1Addr := strings.Replace(stdout, "'", "", -1)

	// Get the IP address of the second web server.
	stdout, _, err = runWithOpts("docker",
		runOptions{printStdout: true, printStderr: true},
		"inspect", "http2", "-f", "'{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'")
	g.Expect(err).ToNot(gomega.HaveOccurred(), "failed to get IP address of second web server")
	http2Addr := strings.Replace(stdout, "'", "", -1)

	renderConfig := haproxy.NewRenderConfiguration().
		WithDataPlaneConfig(config).
		WithAddresses([]corev1.EndpointAddress{
			{
				IP:       http1Addr,
				NodeName: pointer.StringPtr("http1"),
			},
			{
				IP:       http2Addr,
				NodeName: pointer.StringPtr("http2"),
			},
		})

	haproxyCfg, err := renderConfig.RenderHAProxyConfiguration()
	g.Expect(err).ToNot(gomega.HaveOccurred(), "failed to render new configuration")

	_, resp, err := client.ConfigurationApi.PostHAProxyConfiguration(ctx, haproxyCfg, &hapi.PostHAProxyConfigurationOpts{
		Version: nextVersion,
	})
	g.Expect(err).ToNot(gomega.HaveOccurred(), "failed to post new configuration")
	g.Expect(resp.Body.Close()).To(gomega.Succeed())

	// TODO: Add docker tests that can work with a HTTPS check on 6443
}

func runDocker(arg ...string) error {
	_, _, err := runWithOpts("docker", runOptions{
		printStdout: true,
		printStderr: true,
	}, arg...)
	return err
}

// nolint:unparam
func runWithOpts(name string, opts runOptions, arg ...string) (string, string, error) {
	fmt.Printf("Running %s with '%s'\n", name, strings.Join(arg, " "))
	if resolvedName, _ := exec.LookPath(name); resolvedName != "" {
		name = resolvedName
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.Command(name, arg...)
	cmd.Dir = opts.workDir
	if opts.printStdout {
		cmd.Stdout = io.MultiWriter(stdout, os.Stdout)
	} else {
		cmd.Stdout = stdout
	}
	if opts.printStderr {
		cmd.Stderr = io.MultiWriter(stderr, os.Stderr)
	} else {
		cmd.Stderr = stderr
	}
	if err := cmd.Run(); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), nil
}

type runOptions struct {
	printStdout bool
	printStderr bool
	workDir     string
}

const (
	minTCPPort         = 0
	maxTCPPort         = 65535
	maxReservedTCPPort = 1024
	maxRandTCPPort     = maxTCPPort - (maxReservedTCPPort + 1)
)

var (
	tcpPortRand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// IsTCPPortAvailable returns a flag indicating whether or not a TCP port is
// available.
func IsTCPPortAvailable(port int) bool {
	if port < minTCPPort || port > maxTCPPort {
		return false
	}
	conn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// RandomTCPPort gets a free, random TCP port between 1025-65535. If no free
// ports are available -1 is returned.
func RandomTCPPort() int {
	for i := maxReservedTCPPort; i < maxTCPPort; i++ {
		p := tcpPortRand.Intn(maxRandTCPPort) + maxReservedTCPPort + 1
		if IsTCPPortAvailable(p) {
			return p
		}
	}
	return -1
}
