package vc

const CloudProviderConfig = `
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

const CloudInitUserData = `
#cloud-config
users:
- name: ubuntu
  ssh_authorized_keys:
    - {{ .SSHPublicKey }}
  sudo: ALL=(ALL) NOPASSWD:ALL
  groups: sudo
  shell: /bin/bash
{{- if .TrustedCerts }}
ca-certs:
  trusted:
  {{- range .TrustedCerts }}
  - |
{{ indent 3 (base64Decode .) }}
  {{- end }}
{{- end }}
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

const NodeStartupScript = `
{{ define "install" -}}

apt-get update
apt-get install -y apt-transport-https prips
apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys F76221572C52609D

cat <<EOF > /etc/apt/sources.list.d/k8s.list
deb [arch=amd64] https://apt.dockerproject.org/repo ubuntu-xenial main
EOF

apt-get update

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

DOCKER_VER=$(getversion docker.io 18.06)
KUBELET=$(getversion kubelet ${KUBELET_VERSION}-)
KUBEADM=$(getversion kubeadm ${KUBELET_VERSION}-)
KUBECTL=$(getversion kubectl ${KUBELET_VERSION}-)
apt-get install -y docker.io=${DOCKER_VER}

### TEMPORARY solution
# https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/238
# Currently, ubuntu packaging includes kubernetes-cni 0.7.5 when performing
# apt-get install kubernetes-cni.  Older versions of k8s complains and this
# script fails.  This will install 0.7.5 for k8s 1.14 and higher.  It will
# fall back to 0.6.0 for older versions.

if [[ "${KUBELET_VERSION}" -ge "1.14" ]]; then
	apt-get install -y kubernetes-cni=0.7.*
else
	apt-get install -y kubernetes-cni=0.6.*
fi

apt-get install -y kubelet=${KUBELET} kubeadm=${KUBEADM} kubectl=${KUBECTL}

{{- end }}{{/* end install */}}

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
	"PUBLICIP=`ip route get 8.8.8.8 | awk '{for(i=1; i<=NF; i++) if($i~/src/) print $(i+1)}'`" + `

cat > /etc/systemd/system/kubelet.service.d/20-cloud.conf << EOF
[Service]
Environment="KUBELET_EXTRA_ARGS=--node-ip=${PUBLICIP} --cloud-provider=vsphere ${NODE_LABEL_OPTION} ${NODE_TAINTS_OPTION}"
EOF
# clear the content of the /etc/default/kubelet otherwise in v 1.11.* it causes failure to use the env variable set in the 20-cloud.conf file above
echo > /etc/default/kubelet
systemctl daemon-reload

kubeadm join --token "${TOKEN}" "${MASTER}" --ignore-preflight-errors=all --discovery-token-unsafe-skip-ca-verification

for tries in $(seq 1 60); do
	kubectl --kubeconfig /etc/kubernetes/kubelet.conf annotate --overwrite node $(hostname) cluster.k8s.io/machine=${MACHINE} && break
	sleep 1
done
{{- end }}{{/* end configure */}}
`

const MasterStartupScript = `
{{ define "install" -}}

KUBELET_VERSION={{ .Machine.Spec.Versions.Kubelet }}

curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
touch /etc/apt/sources.list.d/kubernetes.list
sh -c 'echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" > /etc/apt/sources.list.d/kubernetes.list'

apt-get update -y

apt-get install -y \
    socat \
    ebtables \
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

DOCKER_VER=$(getversion docker.io 18.06)
KUBELET=$(getversion kubelet ${KUBELET_VERSION}-)
KUBEADM=$(getversion kubeadm ${KUBELET_VERSION}-)

apt-get install -y docker.io=${DOCKER_VER}

### TEMPORARY solution
# https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/issues/238
# Currently, ubuntu packaging includes kubernetes-cni 0.7.5 when performing
# apt-get install kubernetes-cni.  Older versions of k8s complains and this
# script fails.  This will install 0.7.5 for k8s 1.14 and higher.  It will
# fall back to 0.6.0 for older versions.

if [[ "${KUBELET_VERSION}" -ge "1.14" ]]; then
	apt-get install -y kubernetes-cni=0.7.*
else
	apt-get install -y kubernetes-cni=0.6.*
fi

apt-get install -y kubelet=${KUBELET} kubeadm=${KUBEADM}

mv /usr/bin/kubeadm.dl /usr/bin/kubeadm
chmod a+rx /usr/bin/kubeadm
{{- end }}{{/* end install */}}


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
	"PRIVATEIP=`ip route get 8.8.8.8 | awk '{for(i=1; i<=NF; i++) if($i~/src/) print $(i+1)}'`" + `
echo $PRIVATEIP > /tmp/.ip
` +
	"PUBLICIP=`ip route get 8.8.8.8 | awk '{for(i=1; i<=NF; i++) if($i~/src/) print $(i+1)}'`" + `

cat > /etc/systemd/system/kubelet.service.d/20-cloud.conf << EOF
[Service]
Environment="KUBELET_EXTRA_ARGS=--node-ip=${PUBLICIP} --cloud-provider=vsphere --cloud-config=/etc/kubernetes/cloud-config/cloud-config.yaml ${NODE_LABEL_OPTION} ${NODE_TAINTS_OPTION}"
EOF
# clear the content of the /etc/default/kubelet otherwise in v 1.11.* it causes failure to use the env variable set in the 20-cloud.conf file above
echo > /etc/default/kubelet
systemctl daemon-reload


# Set up kubeadm config file to pass parameters to kubeadm init.

{{ if (or (eq .MajorMinorVersion "1.11") (eq .MajorMinorVersion "1.12")) }}
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

{{- end }}{{/* end MajorMinorVersion 1.11 or 1.12 */}}

{{ if ge .MajorMinorVersion "1.13" }}
cat > /etc/kubernetes/kubeadm_config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
nodeRegistration:
  criSocket: /var/run/dockershim.sock
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
  kubeletExtraArgs:
    cgroup-driver: "cgroupfs"
localAPIEndpoint:
  advertiseAddress: ${PUBLICIP}
  bindPort: ${PORT}
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration
etcd:
  local:
    imageRepository: "k8s.gcr.io"
    imageTag: "3.2.24"
    dataDir: "/var/lib/etcd"
networking:
  serviceSubnet: ${SERVICE_CIDR}
  podSubnet: ${POD_CIDR}
  dnsDomain: ${CLUSTER_DNS_DOMAIN}
kubernetesVersion: v${CONTROL_PLANE_VERSION}
apiServer:
  certSANs:
  - ${PUBLICIP}
  - ${PRIVATEIP}
  extraArgs:
    authorization-mode: Node,RBAC
    cloud-provider: vsphere
    cloud-config: /etc/kubernetes/cloud-config/cloud-config.yaml
  extraVolumes:
  - name: cloud-config
    hostPath: /etc/kubernetes/cloud-config
    mountPath: /etc/kubernetes/cloud-config
    readOnly: true
controllerManager:
  extraArgs:
    cloud-provider: vsphere
    cloud-config: /etc/kubernetes/cloud-config/cloud-config.yaml
    address: 0.0.0.0
  extraVolumes:
  - name: cloud-config
    hostPath: /etc/kubernetes/cloud-config
    mountPath: /etc/kubernetes/cloud-config
    readOnly: true
scheduler:
  extraArgs:
    address: 0.0.0.0
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
clusterDomain: ${CLUSTER_DNS_DOMAIN}
EOF

{{- end }}{{/* end MajorMinorVersion 1.13 */}}


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
	kubectl --kubeconfig /etc/kubernetes/kubelet.conf annotate --overwrite node $(hostname) cluster.k8s.io/machine=${MACHINE} && break
	sleep 1
done

{{- end }}{{/* end configure */}}
`
