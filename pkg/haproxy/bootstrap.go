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

package haproxy

import (
	"bytes"
	"strings"
	"text/template"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

// BootstrapDataForLoadBalancer generates the bootstrap data required
// to bootstrap a new HAProxy VM.
func BootstrapDataForLoadBalancer(
	haProxyLoadBalancer infrav1.HAProxyLoadBalancer,
	username, password,
	signingCertificatePEM, signingCertifiateKey []byte) ([]byte, error) {

	input := struct {
		Username                    string
		Password                    string
		SigningAuthorityCertificate string
		SigningAuthorityKey         string
		User                        *infrav1.SSHUser
		DSMetaHostName              string
		DSMetaLocalIPv4             string
	}{
		Username:                    string(username),
		Password:                    string(password),
		SigningAuthorityCertificate: string(signingCertificatePEM),
		SigningAuthorityKey:         string(signingCertifiateKey),
		DSMetaHostName:              "{{ ds.meta_data.hostname }}",
		DSMetaLocalIPv4:             "{{ ds.meta_data.local_ipv4 }}",
		User:                        haProxyLoadBalancer.Spec.User,
	}

	tpl := template.Must(template.
		New("t").
		Funcs(template.FuncMap{
			"Indent": templateYAMLIndent,
		}).
		Parse(haproxyLoadBalancerBootstrapTemplateFormat))

	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, input); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func templateYAMLIndent(i int, input string) string {
	split := strings.Split(input, "\n")
	ident := "\n" + strings.Repeat(" ", i)
	return strings.Repeat(" ", i) + strings.Join(split, ident)
}
