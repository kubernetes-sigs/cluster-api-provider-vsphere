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

package main

import (
	"fmt"
	"os"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vmw-guestinfo/rpcvmx"
	"github.com/vmware/vmw-guestinfo/vmcheck"

	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vim25/xml"
	"sigs.k8s.io/cluster-api-provider-vsphere-installer/pkg/version"
)

func main() {

	app := cli.NewApp()
	app.Usage = "Fetch OVF environment information"
	app.Version = version.GetBuild().ShortVersion()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "key, k",
			Value: "",
			Usage: "Work on single OVF property",
		},
		cli.StringFlag{
			Name:  "set, s",
			Value: "",
			Usage: "Set value for OVF property",
		},
	}

	app.Action = func(c *cli.Context) error {

		// Check if we're running inside a VM
		isVM, err := vmcheck.IsVirtualWorld()
		if err != nil {
			fmt.Printf("error: %s\n", err.Error())
			return err
		}

		// No point in running if we're not inside a VM
		if !isVM {
			fmt.Println("not living in a virtual world... :(")
			return err
		}

		config := rpcvmx.NewConfig()
		var e ovf.Env

		if err := fetchovfEnv(config, &e); err != nil {
			return err
		}

		// If set and key are populated, let's set the key to the value passed
		if c.String("set") != "" && c.String("key") != "" {

			var props []ovf.EnvProperty

			for _, p := range e.Property.Properties {
				if p.Key == c.String("key") {
					props = append(props, ovf.EnvProperty{
						Key:   p.Key,
						Value: c.String("set"),
					})
				} else {
					props = append(props, ovf.EnvProperty{
						Key:   p.Key,
						Value: p.Value,
					})
				}
			}

			env := ovf.Env{
				EsxID: e.EsxID,
				Platform: &ovf.PlatformSection{
					Kind:    e.Platform.Kind,
					Version: e.Platform.Version,
					Vendor:  e.Platform.Vendor,
					Locale:  e.Platform.Locale,
				},
				Property: &ovf.PropertySection{
					Properties: props,
				},
			}
			// Send updated ovfEnv through the rpcvmx channel
			if err := config.SetString("guestinfo.ovfEnv", env.MarshalManual()); err != nil {
				return err
			}
			// Refresh ovfEnv
			if err := fetchovfEnv(config, &e); err != nil {
				return err
			}

		}

		// LET'S HAVE A MAP! SO YOU CAN DO LOOKUPS!
		m := make(map[string]string)
		for _, v := range e.Property.Properties {
			m[v.Key] = v.Value
		}

		// If a key is all we want...
		if c.String("key") != "" {
			fmt.Println(m[c.String("key")])
			return nil
		}

		// Let's print the whole property list by default
		for k, v := range m {
			fmt.Printf("[%s]=%s\n", k, v)
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}
}

func fetchovfEnv(config *rpcvmx.Config, e *ovf.Env) error {
	ovfEnv, err := config.String("guestinfo.ovfEnv", "")
	if err != nil {
		return fmt.Errorf("impossible to fetch ovf environment, exiting")
	}

	if err = xml.Unmarshal([]byte(ovfEnv), &e); err != nil {
		return fmt.Errorf("error: %s", err.Error())
	}

	return nil
}
