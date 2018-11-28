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

package lib

import (
	"encoding/xml"
	"strings"

	"github.com/vmware/vmw-guestinfo/rpcvmx"
)

// EnvFetchError is returned when the ovf env cannot be fetched via RPC.
type EnvFetchError struct {
	msg string
}

func (e EnvFetchError) Error() string {
	return e.msg
}

// UnmarshalError is returned when the ovf env cannot be unmarshaled.
type UnmarshalError struct {
	msg string
}

func (e UnmarshalError) Error() string {
	return e.msg
}

// Environment stores guestinfo data.
type Environment struct {
	Properties map[string]string
}

// UnmarshaledOvfEnv returns the unmarshaled OVA environment fetched via RPC.
func UnmarshaledOvfEnv() (Environment, error) {
	config := rpcvmx.NewConfig()
	// Fetch OVF Environment via RPC
	ovfEnv, err := config.String("guestinfo.ovfEnv", "")
	if err != nil {
		return Environment{}, EnvFetchError{
			msg: "unable to fetch ovf environment",
		}
	}

	// TODO: fix this when proper support for namespaces is added to golang.
	// ref: golang/go/issues/14407 and golang/go/issues/14407
	ovfEnv = strings.Replace(ovfEnv, "oe:key", "key", -1)
	ovfEnv = strings.Replace(ovfEnv, "oe:value", "value", -1)

	var ovf Environment
	err = xml.Unmarshal([]byte(ovfEnv), &ovf)
	if err != nil {
		return Environment{}, UnmarshalError{
			msg: "unable to unmarshal ovf environment",
		}
	}

	return ovf, nil
}

func (e *Environment) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type property struct {
		Key   string `xml:"key,attr"`
		Value string `xml:"value,attr"`
	}

	type propertySection struct {
		Property []property `xml:"Property"`
	}

	var environment struct {
		XMLName         xml.Name        `xml:"Environment"`
		PropertySection propertySection `xml:"PropertySection"`
	}
	err := d.DecodeElement(&environment, &start)
	if err == nil {
		e.Properties = make(map[string]string)
		for _, v := range environment.PropertySection.Property {
			e.Properties[v.Key] = v.Value
		}
	}
	return err

}
