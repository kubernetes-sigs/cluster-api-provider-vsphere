// Copyright 2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build linux,amd64

package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/coreos/go-systemd/daemon"
	"github.com/spf13/pflag"

	"github.com/vmware/govmomi/toolbox"
	"github.com/vmware/govmomi/toolbox/vix"
	"sigs.k8s.io/cluster-api-provider-vsphere-installer/pkg/version"
)

const (
	keepEnvVars = false
	sdReady     = "READY=1"
)

// This example can be run on a VM hosted by ESX, Fusion or Workstation
func main() {

	log := logrus.New().WithField("app", "toolbox")

	var ver = false
	pflag.BoolVar(&ver, "version", ver, "Show version information")

	pflag.Parse()

	if ver {
		v := version.GetBuild()
		log.Println("version:", v.Version)
		log.Println("commit:", v.GitCommit)
		log.Println("build date:", v.BuildDate)
		os.Exit(0)
	}

	in := toolbox.NewBackdoorChannelIn()
	out := toolbox.NewBackdoorChannelOut()

	service := toolbox.NewService(in, out)

	if os.Getuid() == 0 {
		service.Power.Halt.Handler = toolbox.Halt
		service.Power.Reboot.Handler = toolbox.Reboot
	}

	// Disable all guest operations
	service.Command.Authenticate = func(_ vix.CommandRequestHeader, _ []byte) error {
		return errors.New("not authorized")
	}

	err := service.Start()
	if err != nil {
		log.Fatal(err)
	}

	daemon.SdNotify(keepEnvVars, sdReady)

	// handle the signals and gracefully shutdown the service
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("signal %s received", <-sig)
		service.Stop()
	}()

	service.Wait()
}
