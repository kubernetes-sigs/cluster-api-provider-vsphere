/*
Copyright 2017 The Kubernetes Authors.

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

package common

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type TemplateParams struct {
	Token        string
	Cluster      *clusterv1.Cluster
	Machine      *clusterv1.Machine
	DockerImages []string
	Preloaded    bool
}

// Returns the startup script for the nodes.
func GetNodeStartupScript(params TemplateParams) (string, error) {
	var buf bytes.Buffer
	tName := "fullScript"
	if isPreloaded(params) {
		tName = "preloadedScript"
	}

	if err := nodeStartupScriptTemplate.ExecuteTemplate(&buf, tName, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GetMasterStartupScript(params TemplateParams) (string, error) {
	var buf bytes.Buffer
	tName := "fullScript"
	if isPreloaded(params) {
		tName = "preloadedScript"
	}

	if err := masterStartupScriptTemplate.ExecuteTemplate(&buf, tName, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func isPreloaded(params TemplateParams) bool {
	return params.Preloaded
}

// PreloadMasterScript returns a script that can be used to preload a master.
func PreloadMasterScript(version string, dockerImages []string) (string, error) {
	return preloadScript(masterStartupScriptTemplate, version, dockerImages)
}

// PreloadNodeScript returns a script that can be used to preload a master.
func PreloadNodeScript(version string, dockerImages []string) (string, error) {
	return preloadScript(nodeStartupScriptTemplate, version, dockerImages)
}

func preloadScript(t *template.Template, version string, dockerImages []string) (string, error) {
	var buf bytes.Buffer
	params := TemplateParams{
		Machine:      &clusterv1.Machine{},
		DockerImages: dockerImages,
	}
	params.Machine.Spec.Versions.Kubelet = version
	err := t.ExecuteTemplate(&buf, "generatePreloadedImage", params)
	return buf.String(), err
}

var (
	nodeStartupScriptTemplate        *template.Template
	masterStartupScriptTemplate      *template.Template
	cloudInitUserDataTemplate        *template.Template
	cloudProviderConfigTemplate      *template.Template
	cloudInitMetaDataNetworkTemplate *template.Template
	cloudInitMetaDataTemplate        *template.Template
)

func init() {
	endpoint := func(apiEndpoint *clusterv1.APIEndpoint) string {
		return fmt.Sprintf("%s:%d", apiEndpoint.Host, apiEndpoint.Port)
	}

	labelMap := func(labels map[string]string) string {
		var builder strings.Builder
		for k, v := range labels {
			builder.WriteString(fmt.Sprintf("%s=%s,", k, v))
		}
		return strings.TrimRight(builder.String(), ",")
	}

	taintMap := func(taints []corev1.Taint) string {
		var builder strings.Builder
		for _, taint := range taints {
			builder.WriteString(fmt.Sprintf("%s=%s:%s,", taint.Key, taint.Value, taint.Effect))
		}
		return strings.TrimRight(builder.String(), ",")
	}

	// Force a compliation error if getSubnet changes. This is the
	// signature the templates expect, so changes need to be
	// reflected in templates below.
	var _ func(clusterv1.NetworkRanges) string = vsphereutils.GetSubnet
	funcMap := map[string]interface{}{
		"endpoint":  endpoint,
		"getSubnet": vsphereutils.GetSubnet,
		"labelMap":  labelMap,
		"taintMap":  taintMap,
	}
	nodeStartupScriptTemplate = template.Must(template.New("nodeStartupScript").Funcs(funcMap).Parse(nodeStartupScript))
	nodeStartupScriptTemplate = template.Must(nodeStartupScriptTemplate.Parse(genericTemplates))
	masterStartupScriptTemplate = template.Must(template.New("masterStartupScript").Funcs(funcMap).Parse(masterStartupScript))
	masterStartupScriptTemplate = template.Must(masterStartupScriptTemplate.Parse(genericTemplates))
	cloudInitUserDataTemplate = template.Must(template.New("cloudInitUserData").Parse(cloudInitUserData))
	cloudProviderConfigTemplate = template.Must(template.New("cloudProviderConfig").Parse(cloudProviderConfig))
	cloudInitMetaDataNetworkTemplate = template.Must(template.New("cloudInitMetaDataNetwork").Parse(networkSpec))
	cloudInitMetaDataTemplate = template.Must(template.New("cloudInitMetaData").Parse(cloudInitMetaData))
}

// Returns the startup script for the nodes.
func GetCloudInitMetaData(name string, params *vsphereconfigv1.VsphereMachineProviderConfig) (string, error) {
	var buf bytes.Buffer
	param := CloudInitMetadataNetworkTemplate{
		Networks: params.MachineSpec.Networks,
	}
	if err := cloudInitMetaDataNetworkTemplate.Execute(&buf, param); err != nil {
		return "", err
	}
	param2 := CloudInitMetadataTemplate{
		NetworkSpec: base64.StdEncoding.EncodeToString(buf.Bytes()),
		Hostname:    name,
	}
	buf.Reset()
	if err := cloudInitMetaDataTemplate.Execute(&buf, param2); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Returns the startup script for the nodes.
func GetCloudInitUserData(params CloudInitTemplate) (string, error) {
	var buf bytes.Buffer

	if err := cloudInitUserDataTemplate.Execute(&buf, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Returns the startup script for the nodes.
func GetCloudProviderConfigConfig(params CloudProviderConfigTemplate) (string, error) {
	var buf bytes.Buffer

	if err := cloudProviderConfigTemplate.Execute(&buf, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type CloudProviderConfigTemplate struct {
	Datacenter   string
	Server       string
	Insecure     bool
	UserName     string
	Password     string
	ResourcePool string
	Datastore    string
	Network      string
}

type CloudInitTemplate struct {
	Script              string
	IsMaster            bool
	CloudProviderConfig string
	SSHPublicKey        string
}

type CloudInitMetadataNetworkTemplate struct {
	Networks []vsphereconfigv1.NetworkSpec
}
type CloudInitMetadataTemplate struct {
	NetworkSpec string
	Hostname    string
}

const cloudInitMetaData = `
{
  "network": "{{ .NetworkSpec }}",
  "network.encoding": "base64",
  "local-hostname": "{{ .Hostname }}"
}
`

const networkSpec = `
version: 1
config:
{{- range $index, $network := .Networks}}
  - type: physical
    name: eth{{ $index }}
    subnets:
    {{- if eq $network.IPConfig.NetworkType "static" }}
      - type: static
        address: {{ $network.IPConfig.IP }}
        {{- if $network.IPConfig.Gateway }}
        gateway: {{ $network.IPConfig.Gateway }}
        {{- end }}
        {{- if $network.IPConfig.Netmask }}
        netmask: {{ $network.IPConfig.Netmask }}
        {{- end }}
        {{- if $network.IPConfig.Dns }}
        dns_nameservers:
        {{- range $network.IPConfig.Dns }}
          - {{ . }}
        {{- end }}
        {{- end }}
    {{- else }}
      - type: dhcp
    {{- end }}
{{- end }}
`

const cloudInitUserData = `
#cloud-config
users:
- name: ubuntu
  ssh_authorized_keys:
    - {{ .SSHPublicKey }}
  sudo: ALL=(ALL) NOPASSWD:ALL
  groups: sudo
  shell: /bin/bash
write_files:
  - path: /tmp/boot.sh
    content: |
      {{ .Script }}
    permissions: '0755'
    encoding: base64
  {{- if .IsMaster }}
  - path: /etc/kubernetes/cloud-config/cloud-config.yaml
    content: |
      {{ .CloudProviderConfig }}
    permissions: '0600'
    encoding: base64
  {{- end }}
runcmd:
  - /tmp/boot.sh
`

const cloudProviderConfig = `
[Global]
datacenters = "{{ .Datacenter }}"
insecure-flag = "{{ if .Insecure }}1{{ else }}0{{ end }}" #set to 1 if the vCenter uses a self-signed cert

[VirtualCenter "{{ .Server }}"]
        user = "{{ .UserName }}"
        password = "{{ .Password }}"

[Workspace]
        server = "{{ .Server }}"
        datacenter = "{{ .Datacenter }}"
        folder = "{{ .ResourcePool }}"
        default-datastore = "{{ .Datastore }}"
        resourcepool-path = "{{ .ResourcePool }}"

[Disk]
        scsicontrollertype = pvscsi

[Network]
        public-network = "{{ .Network }}"
`

const genericTemplates = `
{{ define "fullScript" -}}
  {{ template "startScript" . }}
  {{ template "install" . }}
  {{ template "configure" . }}
  {{ template "endScript" . }}
{{- end }}

{{ define "preloadedScript" -}}
  {{ template "startScript" . }}
  {{ template "configure" . }}
  {{ template "endScript" . }}
{{- end }}

{{ define "generatePreloadedImage" -}}
  {{ template "startScript" . }}
  {{ template "install" . }}

systemctl enable docker || true
systemctl start docker || true

  {{ range .DockerImages }}
docker pull {{ . }}
  {{ end  }}

  {{ template "endScript" . }}
{{- end }}

{{ define "startScript" -}}
#!/bin/bash

set -e
set -x

(
{{- end }}

{{define "endScript" -}}

echo done.
) 2>&1 | tee /var/log/startup.log

{{- end }}
`

const nodeStartupScript = `
{{ define "install" -}}

apt-get update
apt-get install -y apt-transport-https prips
apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys F76221572C52609D

cat <<EOF > /etc/apt/sources.list.d/k8s.list
deb [arch=amd64] https://apt.dockerproject.org/repo ubuntu-xenial main
EOF

apt-get update
apt-get install -y docker.io

curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -

cat <<EOF > /etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update

KUBELET_VERSION={{ .Machine.Spec.Versions.Kubelet }}
# Our Debian packages have versions like "1.8.0-00" or "1.8.0-01". Do a prefix
# search based on our SemVer to find the right (newest) package version.
function getversion() {
	name=$1
	prefix=$2
	version=$(apt-cache madison $name | awk '{ print $3 }' | grep ^$prefix | head -n1)
	if [[ -z "$version" ]]; then
		echo Can\'t find package $name with prefix $prefix
		exit 1
	fi
	echo $version
}

KUBELET=$(getversion kubelet ${KUBELET_VERSION}-)
KUBEADM=$(getversion kubeadm ${KUBELET_VERSION}-)
KUBECTL=$(getversion kubectl ${KUBELET_VERSION}-)
# Explicit cni version is a temporary workaround till the right version can be automatically detected correctly
apt-get install -y kubelet=${KUBELET} kubeadm=${KUBEADM} kubectl=${KUBECTL}

{{- end }} {{/* end install */}}

{{ define "configure" -}}
TOKEN={{ .Token }}
MASTER={{ index .Cluster.Status.APIEndpoints 0 | endpoint }}
MACHINE={{ .Machine.ObjectMeta.Namespace }}/{{ .Machine.ObjectMeta.Name }}
NODE_LABEL_OPTION={{ if .Machine.Spec.Labels }}--node-labels={{ labelMap .Machine.Spec.Labels }}{{ end }}
NODE_TAINTS_OPTION={{ if .Machine.Spec.Taints }}--register-with-taints={{ taintMap .Machine.Spec.Taints }}{{ end }}

# Disable swap otherwise kubelet won't run
swapoff -a
sed -i '/ swap / s/^/#/' /etc/fstab

systemctl enable docker || true
systemctl start docker || true

sysctl net.bridge.bridge-nf-call-iptables=1

` +
	"PUBLICIP=`ip route get 8.8.8.8 | awk '{printf \"%s\", $NF; exit}'`" + `

cat > /etc/systemd/system/kubelet.service.d/20-cloud.conf << EOF
[Service]
Environment="KUBELET_EXTRA_ARGS=--node-ip=${PUBLICIP} --cloud-provider=vsphere ${NODE_LABEL_OPTION} ${NODE_TAINTS_OPTION}"
EOF
# clear the content of the /etc/default/kubelet otherwise in v 1.11.* it causes failure to use the env variable set in the 20-cloud.conf file above
echo > /etc/default/kubelet
systemctl daemon-reload

# (gjayavelu) Begin workaround to avoid asymetric routing issue on worker nodes due to multiple interfaces
# and each having their own gateways.
cat > readInterfaces.awk << EOF
BEGIN {

    start = 0;
    outAll = 0

    for (i = 2; i < ARGC; i++) {
        split(ARGV[i], arg, "=");
        if (arg[1] == "device")
            device = arg[2];
        else if (arg[1] == "output" && arg[2] == "all")
            outAll = 1;
        else if (arg[1] == "debug" && arg[2] == "1")
            debug = 1;
    }

    if (!length(device)) {
        print "awk -f readInterfaces.awk <interfaces file> device=<eth device> [output=all] [debug=1]"
        exit 1;
    }
}

{
    # Look for iface line and if the interface comes with the device name
    # scan whether it is dhcp or static or manual
    # e.g. iface eth0 inet [static | dhcp | manual]
    if (\$1 == "iface")  {
        # Ethernet name matches - switch the line scanning on
        if (\$2 == device) {
            if (debug)
                print \$0;
            # It's a DHCP interface
            if (match(\$0, / dhcp/)) {
                print "dhcp";
                gotTypeNoAddr = 1;
                exit 0;
                # It's a static network interface. We want to scan the
                # addresses after the static line
            }
            else if (match (\$0, / static/)) {
                static = 1;
                next;
            }
            else if (match (\$0, / manual/)) {
                print "manual";
                gotTypeNoAddr = 1;
                exit 0;
            }
            # If it is other inteface line, switch it off
            # Go to the next line
        }
        else {
            static = 0;
            next;
        }
    }
    else if (\$1 == "auto") {
        static = 0;
        next;
    }

    # At here, it means we are after the iface static line of
    # after the device we are searching for
    # Scan for the static content
    if (static) {
        # 2nd field to end of the line
        if (length(\$1)) {
            interface[\$1] = substr(\$0, index(\$0, \$2));
            gotAddr = 1;
        }
    }
}

END {
    if (gotAddr) {
        printf("%s %s %s\n", interface["address"], interface["netmask"], interface["gateway"]);
        if (outAll) {
            delete interface["address"];
            delete interface["netmask"];
            delete interface["gateway"];
            for (field in interface) {
                printf("%s %s\n", field, interface[field]);
            }
        }
        exit 0;
    }
    else {
        if (gotTypeNoAddr)
            exit 0;
        else
            exit 1;
    }
}
EOF

toaddr() {
    b1=$(( ($1 & 0xFF000000) >> 24))
    b2=$(( ($1 & 0xFF0000) >> 16))
    b3=$(( ($1 & 0xFF00) >> 8))
    b4=$(( $1 & 0xFF ))
    eval "$2=\$b1.\$b2.\$b3.\$b4"
}

tonum() {
    if [[ $1 =~ ([[:digit:]]+)\.([[:digit:]]+)\.([[:digit:]]+)\.([[:digit:]]+) ]]; then
        addr=$(( (${BASH_REMATCH[1]} << 24) + (${BASH_REMATCH[2]} << 16) + (${BASH_REMATCH[3]} << 8) + ${BASH_REMATCH[4]} ))
        eval "$2=\$addr"
    fi
}

configure_routes() {
    rt_tables_file=/etc/iproute2/rt_tables
    grep -q -F "$1 $2-table" $rt_tables_file || echo $1 $2-table >> $rt_tables_file
    ip rule add from $3 lookup $2-table >>/tmp/routes 2>&1
    ip route add $3 dev $2 table $2-table >>/tmp/routes 2>&1
    if [ -n "$4" ]; then
      ip route add default via $4 table $2-table >>/tmp/routes 2>&1
    fi
}
get_network_info() {
    if [[ $1 =~ ^([0-9\.]+)/([0-9]+)$ ]]; then
      ip_addr=${BASH_REMATCH[1]}
      prefix_len=${BASH_REMATCH[2]}
      zeros=$((32-prefix_len))
      netmask_num=0
      for (( i=0; i<$zeros; i++ )); do
          netmask_num=$(( (netmask_num << 1) ^ 1 ))
      done
      netmask_num=$((netmask_num ^ 0xFFFFFFFF))
      tonum $ip_addr ip_addr_num
      tonum $netmask netmask_num

      network_num=$(( ip_addr_num & netmask_num ))

      toaddr $network_num network
      eval "$2=\$network/$prefix_len"
    fi
}

entry_num=100
network_script=/etc/network/interfaces.d/50-cloud-init.cfg

rt_tables=()
for iface in $(ifconfig | cut -d ' ' -f1| tr ':' '\n' | awk NF | grep ^eth)
do
   net_info=$(awk -f readInterfaces.awk $network_script device=$iface)
   IFS=' ' read -r -a array <<< "$net_info"
   if [ "$net_info" = "dhcp" ]; then
     int_addr=$(ip -f inet addr show $iface | grep -Po 'inet \K[\d.]+/[\d.]+')
     dhcp_lease=/var/lib/dhcp/dhclient.$iface.leases
     if [ -f "$dhcp_lease" ]; then
       gateway=$(grep "option routers" $dhcp_lease | tail -n 1 | awk '{print $3}' | sed -e 's/;//')
     fi
   elif [ "${#array[@]}" -ge "1" ]; then
     gateway=${array[1]}
     int_addr=${array[0]}
   fi

   get_network_info $int_addr network
   rt_tables+=($iface)
   configure_routes $entry_num $iface $network $gateway
   let "entry_num++"
done
# End of workaround

kubeadm join --token "${TOKEN}" "${MASTER}" --skip-preflight-checks --discovery-token-unsafe-skip-ca-verification

for tries in $(seq 1 60); do
	kubectl --kubeconfig /etc/kubernetes/kubelet.conf annotate --overwrite node $(hostname) machine=${MACHINE} && break
	sleep 1
done

# (gjayavelu) Begin workaround to configure route for weave network
weave_addr=""
while [[ -z "$weave_addr" ]];
do
  sleep 1
  printf "Waiting for weave interface\n"
  weave_addr=$((ip -f inet addr show weave || true) | (grep -Po 'inet \K[\d.]+/[\d.]+' || true))
done
get_network_info $weave_addr weave_net
for iface in "${rt_tables[@]}"
do
    ip route add $weave_net dev weave table $iface-table >>/tmp/routes 2>&1
done
# End of workaround
{{- end }} {{/* end configure */}}
`

const masterStartupScript = `
{{ define "install" -}}

KUBELET_VERSION={{ .Machine.Spec.Versions.Kubelet }}

curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
touch /etc/apt/sources.list.d/kubernetes.list
sh -c 'echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" > /etc/apt/sources.list.d/kubernetes.list'

apt-get update -y

apt-get install -y \
    socat \
    ebtables \
    docker.io \
    apt-transport-https \
    cloud-utils \
    prips

export VERSION=v${KUBELET_VERSION}
export ARCH=amd64
curl -sSL https://dl.k8s.io/release/${VERSION}/bin/linux/${ARCH}/kubeadm > /usr/bin/kubeadm.dl
chmod a+rx /usr/bin/kubeadm.dl

# Our Debian packages have versions like "1.8.0-00" or "1.8.0-01". Do a prefix
# search based on our SemVer to find the right (newest) package version.
function getversion() {
	name=$1
	prefix=$2
	version=$(apt-cache madison $name | awk '{ print $3 }' | grep ^$prefix | head -n1)
	if [[ -z "$version" ]]; then
		echo Can\'t find package $name with prefix $prefix
		exit 1
	fi
	echo $version
}

KUBELET=$(getversion kubelet ${KUBELET_VERSION}-)
KUBEADM=$(getversion kubeadm ${KUBELET_VERSION}-)

# Explicit cni version is a temporary workaround till the right version can be automatically detected correctly
apt-get install -y \
    kubelet=${KUBELET} \
    kubeadm=${KUBEADM} 

mv /usr/bin/kubeadm.dl /usr/bin/kubeadm
chmod a+rx /usr/bin/kubeadm
{{- end }} {{/* end install */}}


{{ define "configure" -}}
PORT=443
MACHINE={{ .Machine.ObjectMeta.Name }}
CONTROL_PLANE_VERSION={{ .Machine.Spec.Versions.ControlPlane }}
CLUSTER_DNS_DOMAIN={{ .Cluster.Spec.ClusterNetwork.ServiceDomain }}
POD_CIDR={{ getSubnet .Cluster.Spec.ClusterNetwork.Pods }}
SERVICE_CIDR={{ getSubnet .Cluster.Spec.ClusterNetwork.Services }}
NODE_LABEL_OPTION={{ if .Machine.Spec.Labels }}--node-labels={{ labelMap .Machine.Spec.Labels }}{{ end }}
NODE_TAINTS_OPTION={{ if .Machine.Spec.Taints }}--register-with-taints={{ taintMap .Machine.Spec.Taints }}{{ end }}

# Disable swap otherwise kubelet won't run
swapoff -a
sed -i '/ swap / s/^/#/' /etc/fstab

systemctl enable docker
systemctl start docker

` +
	"PRIVATEIP=`ip route get 8.8.8.8 | awk '{printf \"%s\", $NF; exit}'`" + `
echo $PRIVATEIP > /tmp/.ip
` +
	"PUBLICIP=`ip route get 8.8.8.8 | awk '{printf \"%s\", $NF; exit}'`" + `

cat > /etc/systemd/system/kubelet.service.d/20-cloud.conf << EOF
[Service]
Environment="KUBELET_EXTRA_ARGS=--node-ip=${PUBLICIP} --cloud-provider=vsphere --cloud-config=/etc/kubernetes/cloud-config/cloud-config.yaml ${NODE_LABEL_OPTION} ${NODE_TAINTS_OPTION}"
EOF
# clear the content of the /etc/default/kubelet otherwise in v 1.11.* it causes failure to use the env variable set in the 20-cloud.conf file above
echo > /etc/default/kubelet
systemctl daemon-reload

# Set up kubeadm config file to pass parameters to kubeadm init.
cat > /etc/kubernetes/kubeadm_config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1alpha2
kind: MasterConfiguration
api:
  advertiseAddress: ${PUBLICIP}
  bindPort: ${PORT}
networking:
  serviceSubnet: ${SERVICE_CIDR}
  podSubnet: ${POD_CIDR}
  dnsDomain: ${CLUSTER_DNS_DOMAIN}
kubernetesVersion: v${CONTROL_PLANE_VERSION}
apiServerCertSANs:
- ${PUBLICIP}
- ${PRIVATEIP}
apiServerExtraArgs:
  cloud-provider: vsphere
  cloud-config: /etc/kubernetes/cloud-config/cloud-config.yaml
apiServerExtraVolumes:
  - name: cloud-config
    hostPath: /etc/kubernetes/cloud-config
    mountPath: /etc/kubernetes/cloud-config
controllerManagerExtraArgs:
  cloud-provider: vsphere
  cloud-config: /etc/kubernetes/cloud-config/cloud-config.yaml
  address: 0.0.0.0
schedulerExtraArgs:
  address: 0.0.0.0
controllerManagerExtraVolumes:
  - name: cloud-config
    hostPath: /etc/kubernetes/cloud-config
    mountPath: /etc/kubernetes/cloud-config
kubeletConfiguration:
  baseConfig:
    clusterDomain: ${CLUSTER_DNS_DOMAIN}
nodeRegistration:
  criSocket: /var/run/dockershim.sock
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
EOF

kubeadm init --config /etc/kubernetes/kubeadm_config.yaml

# install weave
cat > /tmp/weave.yaml << EOF
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: weave-net
      labels:
        name: weave-net
      namespace: kube-system
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: ClusterRole
    metadata:
      name: weave-net
      labels:
        name: weave-net
    rules:
      - apiGroups:
          - ''
        resources:
          - pods
          - namespaces
          - nodes
        verbs:
          - get
          - list
          - watch
      - apiGroups:
          - networking.k8s.io
        resources:
          - networkpolicies
        verbs:
          - get
          - list
          - watch
      - apiGroups:
          - ''
        resources:
          - nodes/status
        verbs:
          - patch
          - update
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: ClusterRoleBinding
    metadata:
      name: weave-net
      labels:
        name: weave-net
    roleRef:
      kind: ClusterRole
      name: weave-net
      apiGroup: rbac.authorization.k8s.io
    subjects:
      - kind: ServiceAccount
        name: weave-net
        namespace: kube-system
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: Role
    metadata:
      name: weave-net
      labels:
        name: weave-net
      namespace: kube-system
    rules:
      - apiGroups:
          - ''
        resourceNames:
          - weave-net
        resources:
          - configmaps
        verbs:
          - get
          - update
      - apiGroups:
          - ''
        resources:
          - configmaps
        verbs:
          - create
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: RoleBinding
    metadata:
      name: weave-net
      labels:
        name: weave-net
      namespace: kube-system
    roleRef:
      kind: Role
      name: weave-net
      apiGroup: rbac.authorization.k8s.io
    subjects:
      - kind: ServiceAccount
        name: weave-net
        namespace: kube-system
  - apiVersion: extensions/v1beta1
    kind: DaemonSet
    metadata:
      name: weave-net
      labels:
        name: weave-net
      namespace: kube-system
    spec:
      minReadySeconds: 5
      template:
        metadata:
          labels:
            name: weave-net
        spec:
          containers:
            - name: weave
              command:
                - /home/weave/launch.sh
              env:
                - name: IPALLOC_RANGE
                  value: ${POD_CIDR}
                - name: HOSTNAME
                  valueFrom:
                    fieldRef:
                      apiVersion: v1
                      fieldPath: spec.nodeName
              image: 'weaveworks/weave-kube:2.4.0'
              livenessProbe:
                httpGet:
                  host: 127.0.0.1
                  path: /status
                  port: 6784
                initialDelaySeconds: 30
              resources:
                requests:
                  cpu: 10m
              securityContext:
                privileged: true
              volumeMounts:
                - name: weavedb
                  mountPath: /weavedb
                - name: cni-bin
                  mountPath: /host/opt
                - name: cni-bin2
                  mountPath: /host/home
                - name: cni-conf
                  mountPath: /host/etc
                - name: dbus
                  mountPath: /host/var/lib/dbus
                - name: lib-modules
                  mountPath: /lib/modules
                - name: xtables-lock
                  mountPath: /run/xtables.lock
            - name: weave-npc
              args: []
              env:
                - name: HOSTNAME
                  valueFrom:
                    fieldRef:
                      apiVersion: v1
                      fieldPath: spec.nodeName
              image: 'weaveworks/weave-npc:2.4.0'
              resources:
                requests:
                  cpu: 10m
              securityContext:
                privileged: true
              volumeMounts:
                - name: xtables-lock
                  mountPath: /run/xtables.lock
          hostNetwork: true
          hostPID: true
          restartPolicy: Always
          securityContext:
            seLinuxOptions: {}
          serviceAccountName: weave-net
          tolerations:
            - effect: NoSchedule
              operator: Exists
          volumes:
            - name: weavedb
              hostPath:
                path: /var/lib/weave
            - name: cni-bin
              hostPath:
                path: /opt
            - name: cni-bin2
              hostPath:
                path: /home
            - name: cni-conf
              hostPath:
                path: /etc
            - name: dbus
              hostPath:
                path: /var/lib/dbus
            - name: lib-modules
              hostPath:
                path: /lib/modules
            - name: xtables-lock
              hostPath:
                path: /run/xtables.lock
                type: FileOrCreate
      updateStrategy:
        type: RollingUpdate
EOF

kubectl apply --kubeconfig /etc/kubernetes/admin.conf -f /tmp/weave.yaml

for tries in $(seq 1 60); do
	kubectl --kubeconfig /etc/kubernetes/kubelet.conf annotate --overwrite node $(hostname) machine=${MACHINE} && break
	sleep 1
done

{{- end }} {{/* end configure */}}
`
