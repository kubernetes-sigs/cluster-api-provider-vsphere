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

package helpers

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi/simulator"

	// run init func to register the tagging API endpoints.
	_ "github.com/vmware/govmomi/vapi/simulator"
)

type Simulator struct {
	model  *simulator.Model
	server *simulator.Server
}

func (s Simulator) Destroy() {
	s.server.Close()
	s.model.Remove()
}

func (s Simulator) ServerURL() *url.URL {
	return s.server.URL
}

type Builder struct {
	model      *simulator.Model
	operations []string
}

func VCSimBuilder() *Builder {
	return &Builder{model: simulator.VPX()}
}

func (b *Builder) WithModel(model *simulator.Model) *Builder {
	b.model = model
	return b
}

func (b *Builder) WithOperations(ops ...string) *Builder {
	b.operations = append(b.operations, ops...)
	return b
}

func (b *Builder) Build() (*Simulator, error) {
	err := b.model.Create()
	if err != nil {
		return nil, err
	}

	b.model.Service.TLS = new(tls.Config)
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
	cmd := exec.Command(govcBinPath, args...)
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

func (s Simulator) Run(commandStr string, buffers ...*gbytes.Buffer) error {
	pwd, _ := s.server.URL.User.Password()
	govcURL := fmt.Sprintf("https://%s:%s@%s", s.server.URL.User.Username(), pwd, s.server.URL.Host)

	cmd := govcCommand(govcURL, commandStr, buffers...)
	return cmd.Run()
}

func (s Simulator) Username() string {
	return s.server.URL.User.Username()
}

func (s Simulator) Password() string {
	pwd, _ := s.server.URL.User.Password()
	return pwd
}
