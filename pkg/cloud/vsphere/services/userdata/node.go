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

package userdata

const (
	nodeCloudInit = `{{.Header}}
{{if .SSHAuthorizedKeys}}ssh_authorized_keys:{{range .SSHAuthorizedKeys}}
- "{{.}}"{{end}}{{end}}

runcmd:
-   [hostname, {{HostNameLookup}}]

write_files:
-   path: /etc/hostname
    owner: root:root
    permissions: 0644
    content: |
      {{ HostNameLookup }}

-   path: /etc/hosts
    owner: root:root
    permissions: 0644
    content: |
      ::1         ipv6-localhost ipv6-loopback
      127.0.0.1   localhost
      127.0.0.1   {{HostNameLookup}}

-   path: /tmp/kubeadm-node.yaml
    owner: root:root
    permissions: '0640'
    content: |
      ---
{{.JoinConfiguration | Indent 6}}
kubeadm:
  operation: join
  config: /tmp/kubeadm-node.yaml
`
)

// NodeInput defines the context to generate a node user data.
type NodeInput struct {
	baseUserData

	SSHAuthorizedKeys []string
	JoinConfiguration string
}

// NewNode returns the user data string to be used on a node instance.
func NewNode(input *NodeInput) (string, error) {
	input.Header = cloudConfigHeader
	return generateWithFuncs("node", nodeCloudInit, defaultFuncMap(), input)
}
