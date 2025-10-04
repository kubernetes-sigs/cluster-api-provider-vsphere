/*
Copyright 2025 The Kubernetes Authors.

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

package kubernetes

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

const (
	serviceCIDR = "10.96.0.0/16"
	podCIDR     = "10.244.0.0/16"
	dnsDomain   = "cluster.local"
)

// caSecretHandler implement handling for the secrets storing the control plane certificate authorities.
type caSecretHandler struct {
	// TODO: in a follow up iteration we want to make it possible to store those objects in a dedicate ns on a separated cluster
	//  this brings in the limitation that objects for two clusters with the same name cannot be hosted in a single namespace as well as the need to rethink owner references.
	client client.Client

	cluster           *clusterv1beta1.Cluster
	virtualMachine    client.Object
	virtualMachineGVK schema.GroupVersionKind
}

func (ca *caSecretHandler) LookupOrGenerate(ctx context.Context) error {
	certificates := secret.NewCertificatesForInitialControlPlane(&bootstrapv1.ClusterConfiguration{})

	// Generate cluster certificates on the management cluster if not already there.
	// Note: the code is taking care of service cleanup during the deletion workflow,
	// so this controllerRef is mostly used to express a semantic relation.
	controllerRef := metav1.NewControllerRef(ca.virtualMachine, ca.virtualMachineGVK)
	if err := certificates.LookupOrGenerate(ctx, ca.client, client.ObjectKeyFromObject(ca.cluster), *controllerRef); err != nil {
		return errors.Wrap(err, "failed to generate cluster certificates on the management cluster")
	}

	// TODO: generate certificates on the backing cluster, they are required by generate files

	return nil
}

// kubeConfigSecretHandler implement handling for the secret storing the cluster admin kubeconfig.
type kubeConfigSecretHandler struct {
	// TODO: in a follow up iteration we want to make it possible to store those objects in a dedicate ns on a separated cluster
	//  this brings in the limitation that objects for two clusters with the same name cannot be hosted in a single namespace as well as the need to rethink owner references.
	client client.Client

	cluster           *clusterv1beta1.Cluster
	virtualMachine    client.Object
	virtualMachineGVK schema.GroupVersionKind
}

func (ca *kubeConfigSecretHandler) LookupOrGenerate(ctx context.Context) error {
	// If the secret with the KubeConfig already exists, then no-op.
	if k, _ := secret.GetFromNamespacedName(ctx, ca.client, client.ObjectKeyFromObject(ca.cluster), secret.Kubeconfig); k != nil {
		return nil
	}

	// Otherwise it is required to generate the secret storing the cluster admin kubeconfig.
	// Note: the code is taking care of service cleanup during the deletion workflow,
	// so this controllerRef is mostly used to express a semantic relation.
	controllerRef := metav1.NewControllerRef(ca.virtualMachine, ca.virtualMachineGVK)
	if err := kubeconfig.CreateSecretWithOwner(ctx, ca.client, client.ObjectKeyFromObject(ca.cluster), ca.cluster.Spec.ControlPlaneEndpoint.String(), *controllerRef); err != nil {
		return errors.Wrap(err, "failed to generate cluster certificates on the management cluster")
	}
	return nil
}

// controlPlanePodHandler implement handling for the Pod implementing a control plane.
type controlPlanePodHandler struct {
	// TODO: in a follow up iteration we want to make it possible to store those objects in a dedicate ns on a separated cluster
	//  this brings in the limitation that objects for two clusters with the same name cannot be hosted in a single namespace as well as the need to rethink owner references.
	client client.Client

	controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint
	cluster              *clusterv1beta1.Cluster
	virtualMachine       client.Object
	virtualMachineGVK    schema.GroupVersionKind
}

func (p *controlPlanePodHandler) LookupAndGenerateRBAC(ctx context.Context) error {
	// TODO: think about cleanup or comment that cleanup of RBAC rules won't happen.
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.virtualMachine.GetNamespace(),
			Name:      "kubemark-control-plane",
		},
		Rules: []rbacv1.PolicyRule{
			{
				// TODO: consider if to restrict this somehow
				Verbs:     []string{"get"},
				APIGroups: []string{""}, // "" indicates the core API group
				Resources: []string{"secrets"},
			},
		},
	}
	if err := p.client.Get(ctx, client.ObjectKeyFromObject(role), role); err != nil {
		switch {
		case apierrors.IsNotFound(err):
			if err := p.client.Create(ctx, role); err != nil {
				return errors.Wrap(err, "failed to create kubemark-control-plane Role")
			}
			break
		case apierrors.IsAlreadyExists(err):
			break
		default:
			return errors.Wrap(err, "failed to get kubemark-control-plane Role")
		}
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.virtualMachine.GetNamespace(),
			Name:      "kubemark-control-plane",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				APIGroup: "rbac.authorization.k8s.io",
				// TODO: create a service account and use it here instead of default + use it in the Pod
				Name:      "system:serviceaccount:default:default",
				Namespace: p.virtualMachine.GetNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "kubemark-control-plane",
		},
	}
	if err := p.client.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBinding); err != nil {
		switch {
		case apierrors.IsNotFound(err):
			if err := p.client.Create(ctx, roleBinding); err != nil {
				return errors.Wrap(err, "failed to create kubemark-control-plane RoleBinding")
			}
			break
		case apierrors.IsAlreadyExists(err):
			break
		default:
			return errors.Wrap(err, "failed to get kubemark-control-plane RoleBinding")
		}
	}
	return nil
}

func (p *controlPlanePodHandler) Generate(ctx context.Context, kubernetesVersion string) error {
	// Gets info about the Pod is running the manager in.
	managerPodNamespace := os.Getenv("POD_NAMESPACE")
	managerPodName := os.Getenv("POD_NAME")
	managerPodUID := types.UID(os.Getenv("POD_UID"))

	// Gets the Pod is running the manager in from the management cluster and validate it is the right one.
	managerPod := &corev1.Pod{}
	managerPodKey := types.NamespacedName{Namespace: managerPodNamespace, Name: managerPodName}
	if err := p.client.Get(ctx, managerPodKey, managerPod); err != nil {
		return errors.Wrap(err, "failed to get manager pod")
	}
	if managerPod.UID != managerPodUID {
		return errors.Errorf("manager pod UID does not match, expected %s, got %s", managerPodUID, managerPod.UID)
	}

	// Identify the Container is running the manager in, so we can get the image currently in use for the manager.
	managerContainer := &corev1.Container{}
	for i := range managerPod.Spec.Containers {
		c := managerPod.Spec.Containers[i]
		if c.Name == "manager" {
			managerContainer = &c
		}
	}

	if managerContainer == nil {
		return errors.New("failed to get container from manager pod")
	}

	// Generate the control plane Pod in the BackingCluster.
	// TODO: think about owerRef.
	//  Note: the code is taking care of service cleanup during the deletion workflow,
	//  so this ownerRef is mostly used to express a semantic relation.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.virtualMachine.GetNamespace(),
			//  Kubernetes will generate a name with the cluster name as a prefix.
			GenerateName: fmt.Sprintf("%s-control-plane-", p.virtualMachine.GetName()),
			Labels: map[string]string{
				// Following labels will be used to identify the control plane pods later on.
				"control-plane-endpoint.vcsim.infrastructure.cluster.x-k8s.io": p.controlPlaneEndpoint.Name,

				// Useful labels
				clusterv1beta1.ClusterNameLabel:         p.cluster.Name,
				clusterv1beta1.MachineControlPlaneLabel: "",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				// Use an init container to generate all the key, certificates and KubeConfig files
				// required for the control plane to run.
				generateFilesContainer(managerContainer.Image, p.cluster.Name, p.cluster.Spec.ControlPlaneEndpoint.Host),
			},
			Containers: []corev1.Container{
				// Stacked etcd member for this control plane instance.
				etcdContainer(kubernetesVersion),
				// The control plane instance.
				// Note: control plane components are wired up in order to work well with immutable upgrades (each control plane instance is self-contained),
				apiServerContainer(kubernetesVersion),
				schedulerContainer(kubernetesVersion),
				controllerManagerContainer(kubernetesVersion),
				// eventually adds a dubug container with a volume containing all the generated files
				// TODO: add the debug container conditionally, e.g. if there is an annotation on the virtual machine object.
				// debugContainer(),
			},
			PriorityClassName: "system-node-critical",
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: "RuntimeDefault",
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
			Volumes: []corev1.Volume{
				{
					Name: "etcd-data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "etc-kubernetes",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	if err := p.client.Create(ctx, pod); err != nil {
		return errors.Wrap(err, "failed to create control plane pod")
	}

	// Wait for the pod to show up in the cache
	if err := wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := p.client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return errors.Wrap(err, "failed to get newly created control plane pod")
	}
	return nil
}

func (p *controlPlanePodHandler) GetPods(ctx context.Context) (*corev1.PodList, error) {
	options := []client.ListOption{
		client.InNamespace(p.virtualMachine.GetNamespace()),
		client.MatchingLabels{
			"control-plane-endpoint.vcsim.infrastructure.cluster.x-k8s.io": p.controlPlaneEndpoint.GetName(),
		},
	}

	// TODO: live client or wait for cache update ...
	pods := &corev1.PodList{}
	if err := p.client.List(ctx, pods, options...); err != nil {
		return nil, errors.Wrap(err, "failed to list control plane pods")
	}
	return pods, nil
}

func (p *controlPlanePodHandler) Delete(ctx context.Context, podName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.virtualMachine.GetNamespace(),
			Name:      podName,
		},
	}
	if err := p.client.Delete(ctx, pod); err != nil {
		return errors.Wrap(err, "failed to delete control plane pod")
	}
	return nil
}

func generateFilesContainer(managerImage string, clusterName string, controlPaneEndPointHost string) corev1.Container {
	c := corev1.Container{
		Name: "generate-files",
		// Note: we are using the manager instead of another binary for convenience (the manager is already built and packaged
		// into an image that is published during the release process).
		Image:           managerImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/manager",
			"--generate-control-plane-files",
		},
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "status.podIP",
					},
				},
			},
			{
				Name:  "CLUSTER_NAME",
				Value: clusterName,
			},
			{
				Name:  "CONTROL_PLANE_ENDPOINT_HOST",
				Value: controlPaneEndPointHost,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "etc-kubernetes",
				MountPath: "/etc/kubernetes",
			},
		},
	}
	return c
}

func etcdContainer(kubernetesVersion string) corev1.Container {
	var etcdVersion string
	// TODO: mirror map from kubeadm
	switch kubernetesVersion {
	default:
		etcdVersion = "3.5.4-0"
	}

	c := corev1.Container{
		Name:            "etcd",
		Image:           fmt.Sprintf("registry.k8s.io/etcd:%s", etcdVersion),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "status.podIP",
					},
				},
			},
		},
		Command: []string{
			"etcd",
			"--advertise-client-urls=https://$(POD_IP):2379",
			"--cert-file=/etc/kubernetes/pki/etcd/server.crt",
			"--client-cert-auth=true",
			"--data-dir=/var/lib/etcd",
			"--experimental-initial-corrupt-check=true",
			"--experimental-watch-progress-notify-interval=5s",
			"--initial-advertise-peer-urls=https://$(POD_IP):2380",
			"--initial-cluster=$(POD_NAME)=https://$(POD_IP):2380",
			"--key-file=/etc/kubernetes/pki/etcd/server.key",
			"--listen-client-urls=https://127.0.0.1:2379,https://$(POD_IP):2379",
			"--listen-metrics-urls=http://127.0.0.1:2381",
			"--listen-peer-urls=https://$(POD_IP):2380",
			"--name=$(POD_NAME)",
			"--peer-cert-file=/etc/kubernetes/pki/etcd/peer.crt",
			"--peer-client-cert-auth=true",
			"--peer-key-file=/etc/kubernetes/pki/etcd/peer.key",
			"--peer-trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt",
			"--snapshot-count=10000",
			"--trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt",
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "etcd-data",
				MountPath: "/var/lib/etcd",
			},
			{
				Name:      "etc-kubernetes",
				MountPath: "/etc/kubernetes",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "etcd-peer",
				ContainerPort: 2380,
			},
			// TODO: check if we can drop this port
			/*
				{
					Name:          "etcd-client",
					ContainerPort: 2379,
				},
			*/
		},
		// TODO: enable probes
		/*
			StartupProbe: &corev1.Probe{
				FailureThreshold: 24,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/health?serializable=false",
						Port:   intstr.FromInt(2381),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},
			LivenessProbe: &corev1.Probe{
				FailureThreshold: 8,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/health?exclude=NOSPACE&serializable=true",
						Port:   intstr.FromInt(2381),
						Scheme: corev1.URISchemeHTTP,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},
		*/
	}
	return c
}

func apiServerContainer(kubernetesVersion string) corev1.Container {
	c := corev1.Container{
		Name:            "kube-apiserver",
		Image:           fmt.Sprintf("registry.k8s.io/kube-apiserver:%s", kubernetesVersion),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "status.podIP",
					},
				},
			},
		},
		Command: []string{
			"kube-apiserver",
			"--advertise-address=$(POD_IP)",
			"--allow-privileged=true",
			"--authorization-mode=Node,RBAC",
			"--client-ca-file=/etc/kubernetes/pki/ca.crt",
			"--enable-admission-plugins=NodeRestriction",
			"--enable-bootstrap-token-auth=true",
			"--etcd-cafile=/etc/kubernetes/pki/etcd/ca.crt",
			"--etcd-certfile=/etc/kubernetes/pki/apiserver-etcd-client.crt",
			"--etcd-keyfile=/etc/kubernetes/pki/apiserver-etcd-client.key",
			"--etcd-servers=https://127.0.0.1:2379",
			"--kubelet-client-certificate=/etc/kubernetes/pki/apiserver-kubelet-client.crt",
			"--kubelet-client-key=/etc/kubernetes/pki/apiserver-kubelet-client.key",
			"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
			"--proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt",
			"--proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client.key",
			"--requestheader-allowed-names=front-proxy-client",
			"--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt",
			"--requestheader-extra-headers-prefix=X-Remote-Extra-",
			"--requestheader-group-headers=X-Remote-Group",
			"--requestheader-username-headers=X-Remote-User",
			"--runtime-config=", // TODO: What about this?
			"--secure-port=6443",
			fmt.Sprintf("--service-account-issuer=https://kubernetes.default.svc.%s", dnsDomain),
			"--service-account-key-file=/etc/kubernetes/pki/sa.pub",
			"--service-account-signing-key-file=/etc/kubernetes/pki/sa.key",
			fmt.Sprintf("--service-cluster-ip-range=%s", serviceCIDR),
			"--tls-cert-file=/etc/kubernetes/pki/apiserver.crt",
			"--tls-private-key-file=/etc/kubernetes/pki/apiserver.key",
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("250m"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "etc-kubernetes",
				MountPath: "/etc/kubernetes",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "api-server",
				ContainerPort: 6443,
			},
		},
		// TODO: enable probes
		/*
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/livez",
						Port:   intstr.FromInt(6443),
						Scheme: corev1.URISchemeHTTPS,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/readyz",
						Port:   intstr.FromInt(6443),
						Scheme: corev1.URISchemeHTTPS,
					},
				},
				TimeoutSeconds: 15,
				PeriodSeconds:  1,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/livez",
						Port:   intstr.FromInt(6443),
						Scheme: corev1.URISchemeHTTPS,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},
		*/
	}
	return c
}

func schedulerContainer(kubernetesVersion string) corev1.Container {
	c := corev1.Container{
		Name:            "kube-scheduler",
		Image:           fmt.Sprintf("registry.k8s.io/kube-scheduler:%s", kubernetesVersion),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"kube-scheduler",
			"--authentication-kubeconfig=/etc/kubernetes/scheduler.conf",
			"--authorization-kubeconfig=/etc/kubernetes/scheduler.conf",
			"--bind-address=127.0.0.1",
			"--kubeconfig=/etc/kubernetes/scheduler.conf",
			"--leader-elect=true",
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "etc-kubernetes",
				MountPath: "/etc/kubernetes",
			},
		},
		// TODO: enable probes
		/*
			StartupProbe: &corev1.Probe{
				FailureThreshold: 24,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/healthz",
						Port:   intstr.FromInt(10259),
						Scheme: corev1.URISchemeHTTPS,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},
			LivenessProbe: &corev1.Probe{
				FailureThreshold: 8,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/healthz",
						Port:   intstr.FromInt(10259),
						Scheme: corev1.URISchemeHTTPS,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},
		*/
	}
	return c
}

func controllerManagerContainer(kubernetesVersion string) corev1.Container {
	c := corev1.Container{
		Name:            "kube-controller-manager",
		Image:           fmt.Sprintf("registry.k8s.io/kube-controller-manager:%s", kubernetesVersion),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"kube-controller-manager",
			"--allocate-node-cidrs=true",
			"--authentication-kubeconfig=/etc/kubernetes/controller-manager.conf",
			"--authorization-kubeconfig=/etc/kubernetes/controller-manager.conf",
			"--bind-address=127.0.0.1",
			"--client-ca-file=/etc/kubernetes/pki/ca.crt",
			fmt.Sprintf("--cluster-cidr=%s", podCIDR),
			"--cluster-name=kubemark",
			"--cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt",
			"--cluster-signing-key-file=/etc/kubernetes/pki/ca.key",
			"--controllers=*,bootstrapsigner,tokencleaner",
			"--enable-hostpath-provisioner=true",
			"--kubeconfig=/etc/kubernetes/controller-manager.conf",
			"--leader-elect=true",
			"--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt",
			"--root-ca-file=/etc/kubernetes/pki/ca.crt",
			"--service-account-private-key-file=/etc/kubernetes/pki/sa.key",
			fmt.Sprintf("--service-cluster-ip-range=%s", serviceCIDR),
			"--use-service-account-credentials=true",
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("200m"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "etc-kubernetes",
				MountPath: "/etc/kubernetes",
			},
		},
		// TODO: enable probes
		/*
			StartupProbe: &corev1.Probe{
				FailureThreshold: 24,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/healthz",
						Port:   intstr.FromInt(10257),
						Scheme: corev1.URISchemeHTTPS,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},
			LivenessProbe: &corev1.Probe{
				FailureThreshold: 8,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/healthz",
						Port:   intstr.FromInt(10257),
						Scheme: corev1.URISchemeHTTPS,
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      15,
				PeriodSeconds:       10,
			},

		*/
	}
	return c
}

func debugContainer() corev1.Container {
	debugContainer := corev1.Container{
		Name:            "debug",
		Image:           "ubuntu",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sleep", "infinity"},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "etc-kubernetes",
				MountPath: "/etc/kubernetes",
			},
		},
	}
	return debugContainer
}
