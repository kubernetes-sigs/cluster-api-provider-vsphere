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

// Package vcsim contains tools for running a VCenter simulator.
package vcsim

import (
	"fmt"
	"net/url"

	"github.com/onsi/gomega/gbytes"
	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator" // run init func to register the tagging API endpoints.
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
