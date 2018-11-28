// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
//
// +build linux

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"
	"time"

	"golang.org/x/crypto/ssh"

	ui "github.com/gizak/termui"

	"github.com/dustin/go-humanize"

	"github.com/vmware/vmw-guestinfo/vmcheck"
	"sigs.k8s.io/cluster-api-provider-vsphere/installer/lib"
	"sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/ip"
	"sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/version"
)

const (
	VtActivate   = 0x5606
	VtWaitActive = 0x5607
	refreshTime  = 30 * time.Second
	keyNotAvail  = "SSH key not available"
	socketPath   = "/var/run/dcui.sock"
)

func main() {

	// If we're running under linux, switch to virtual terminal 2 on startup
	ioctl(uintptr(os.Stdout.Fd()), VtActivate, 2)
	ioctl(uintptr(os.Stdout.Fd()), VtWaitActive, 2)

	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()

	gray := ui.ColorRGB(1, 1, 1)
	blue := ui.ColorBlue

	// Check if we're running inside a VM
	if isVM, err := vmcheck.IsVirtualWorld(); err != nil || !isVM {
		fmt.Fprintln(os.Stderr, "not living in a virtual world... :(")
		os.Exit(-1)
	}

	var ovaProps = OVAProps{
		Build:  version.GetBuild().ShortVersion(),
		CPUs:   getCPUs(),
		Memory: getMemory(),
	}

	// Fetch and unmarshal OVF Environment
	ovf, err := lib.UnmarshaledOvfEnv()
	if err != nil {
		switch err.(type) {
		case lib.EnvFetchError:
			ovaProps.Error = "impossible to fetch ovf environment, exiting"
		case lib.UnmarshalError:
			ovaProps.Error = err.Error()
		}
	}

	// Top Panel build
	t := template.Must(template.New("ovaProps").Parse(TopPanelTemplate))

	var topTemplate bytes.Buffer
	t.Execute(&topTemplate, ovaProps)

	toppanel := ui.NewPar(topTemplate.String())
	toppanel.Height = ui.TermHeight()/2 + 1
	toppanel.Width = ui.TermWidth()
	toppanel.TextFgColor = ui.ColorWhite
	toppanel.Y = 0
	toppanel.X = 0
	toppanel.TextBgColor = gray
	toppanel.Bg = gray
	toppanel.BorderBg = gray
	toppanel.BorderFg = ui.ColorWhite
	toppanel.BorderBottom = false
	toppanel.PaddingTop = 4
	toppanel.PaddingLeft = 4

	// Bottom Panel build
	netstat := &NetworkStatus{
		down:     "[MISMATCH](bg-red)",
		up:       "[MATCH](bg-green)",
		ovfProps: ovf.Properties,
	}

	var mainIPAddr net.IP
	mainIPAddr, err = ip.FirstIPv4(ip.Eth0Interface)
	if err != nil {
		mainIPAddr = net.ParseIP("0.0.0.0")
	}

	var currentStatus = OVAStatus{
		KubeStatus:     "[STARTING](fg-yellow)",
		SSHFingerprint: getSSHCertFingerprint(),
		DNS:            netstat.GetDNSStatus(),
		IP:             netstat.GetIPStatus(),
		Gateway:        netstat.GetGatewayStatus(),
	}

	redrawCurrentStatus := func(status string) {
		// Update KubeStatus if not empty
		if status != "" {
			currentStatus.KubeStatus = status
			// If Kubernetes is running, publish SCP instructions
			if strings.Contains(status, "RUNNING") {
				currentStatus.SCPPath = fmt.Sprintf("root@%s:/etc/kubernetes/admin.conf", mainIPAddr.String())
			}
		}

		t := template.Must(template.New("currentStatus").Parse(BottomPanelTemplate))

		var bottomTemplate bytes.Buffer
		t.Execute(&bottomTemplate, currentStatus)

		bottompanel := ui.NewPar(bottomTemplate.String())
		bottompanel.Height = ui.TermHeight() / 2
		bottompanel.Width = ui.TermWidth()
		bottompanel.TextFgColor = gray
		bottompanel.TextBgColor = blue
		bottompanel.Y = ui.TermHeight() / 2
		bottompanel.X = 0
		bottompanel.Bg = blue
		bottompanel.BorderFg = ui.ColorWhite
		bottompanel.BorderBg = blue
		bottompanel.BorderTop = false
		bottompanel.PaddingTop = 1
		bottompanel.PaddingLeft = 4

		ui.Render(toppanel, bottompanel)
	}

	// Redraw the screen every n seconds
	redrawTicker := time.NewTicker(refreshTime)
	go func() {
		for {
			redrawCurrentStatus("")

			<-redrawTicker.C
		}
	}()

	updateKubeStatus := func(c net.Conn) {
		for {
			buf := make([]byte, 512)
			nr, err := c.Read(buf)
			if err != nil {
				return
			}
			data := buf[0:nr]
			redrawCurrentStatus(string(data))
		}
	}

	// Open unix socket to pass KubeStatus messages
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal("Listen error: ", err)
	}
	// TODO(frapposelli): install a signal handler
	defer ln.Close()

	// Handle connections to the socket
	go func() {
		for {
			fd, err := ln.Accept()
			if err != nil {
				log.Fatal("Accept error: ", err)
			}

			go updateKubeStatus(fd)
		}
	}()

	// Quit using "q" on the keyboard
	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ln.Close()
		ui.StopLoop()
	})

	ui.Loop()
}

func ioctl(fd, cmd, ptr uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if e != 0 {
		return e
	}
	return nil
}

func getCPUs() string {
	// #nosec: Subprocess launching should be audited.
	out, _ := exec.Command("/usr/bin/lscpu").Output()
	outstring := strings.TrimSpace(string(out))
	lines := strings.Split(outstring, "\n")
	var cpus string
	var model string
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "CPU(s)":
			cpus = value
		case "Model name":
			model = value
		}
	}

	return fmt.Sprintf("%sx %s", cpus, model)
}

func getMemory() string {
	si := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(si)
	if err != nil {
		panic("Austin, we have a problem... syscall.Sysinfo:" + err.Error())
	}
	return fmt.Sprintf("%s Memory", humanize.IBytes(uint64(si.Totalram)))
}

func getSSHCertFingerprint() string {
	certFile := "/root/.ssh/authorized_keys"
	if _, err := os.Stat(certFile); err == nil {
		certPEM, e := ioutil.ReadFile(certFile)
		if e != nil {
			return fmt.Sprintf("Read error: %s", e.Error())
		}
		key, comment, _, _, err := ssh.ParseAuthorizedKey(certPEM)
		if err != nil {
			return fmt.Sprintf("[SSH KEY PARSE ERROR](bg-red)")
		}
		return fmt.Sprintf("%s %s %s", key.Type(), ssh.FingerprintSHA256(key), comment)
	}
	return keyNotAvail
}
