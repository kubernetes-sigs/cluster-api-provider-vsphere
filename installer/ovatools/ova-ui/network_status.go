// Copyright 2018 The Kubernetes Authors.
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
	"fmt"
	"os/exec"
	"strings"

	"sigs.k8s.io/cluster-api-provider-vsphere/installer/pkg/ip"
)

type NetworkStatus struct {
	down     string
	up       string
	ovfProps map[string]string
}

func (nstat *NetworkStatus) GetDNSStatus() string {
	dnsExpected := strings.FieldsFunc(nstat.ovfProps["network.DNS"],
		func(char rune) bool { return char == ',' || char == ' ' })

	command := `cat /etc/resolv.conf | grep nameserver | awk '{print $2}';`

	return nstat.addressPresenceExec(dnsExpected, command)
}

func (nstat *NetworkStatus) GetIPStatus() string {
	ipsExpected := strings.FieldsFunc(nstat.ovfProps["network.ip0"],
		func(char rune) bool { return char == ',' || char == ' ' })

	ip, err := ip.FirstIPv4(ip.Eth0Interface)
	if err != nil {
		return err.Error()
	}
	return nstat.addressPresenceMatch(ipsExpected, []string{ip.String()})
}

func (nstat *NetworkStatus) GetGatewayStatus() string {
	gatewayExpected := strings.FieldsFunc(nstat.ovfProps["network.gateway"],
		func(char rune) bool { return char == ',' || char == ' ' })

	command := `netstat -nr | grep 0.0.0.0 | head -n 1 | awk '{print $2}';`
	return nstat.addressPresenceExec(gatewayExpected, command)
}

// addressPresenceExec runs a command that returns network information.
func (nstat *NetworkStatus) addressPresenceExec(expectedAddresses []string, command string) string {

	// #nosec: Subprocess launching with variable.
	out, err := exec.Command("/bin/bash", "-c", command).Output()
	if err != nil {
		fmt.Printf("%#v\n%s", err, err.Error())
		return nstat.down
	}
	actualAddresses := strings.Split(strings.TrimSpace(string(out)), "\n")

	return nstat.addressPresenceMatch(expectedAddresses, actualAddresses)
}

// addressPresenceMatch runs a join on two arrays of network addresses to determine if they match
func (nstat *NetworkStatus) addressPresenceMatch(expectedAddresses []string, actualAddresses []string) string {
	if len(expectedAddresses) == 0 {
		return fmt.Sprintf("DHCP -- %s", pretty(actualAddresses))
	}

	// add actual addresses to hashtable
	allAddresses := make(map[string]struct{})
	for _, addr := range actualAddresses {
		allAddresses[addr] = struct{}{}
	}

	// make sure all expected addresses are in hashtable
	for _, addr := range expectedAddresses {
		if _, ok := allAddresses[addr]; !ok {
			return fmt.Sprintf("%s -- OVF set address as: %s but addresses were: %s", nstat.down,
				pretty(expectedAddresses), pretty(actualAddresses))
		}
	}

	return fmt.Sprintf("%s -- %s", nstat.up, pretty(actualAddresses))
}

func pretty(addresses []string) string {
	return strings.Join(addresses, ", ")
}
