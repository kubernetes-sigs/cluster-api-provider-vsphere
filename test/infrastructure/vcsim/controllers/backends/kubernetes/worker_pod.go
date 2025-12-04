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
	"os"
	"time"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/test/infrastructure/kind"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

// workerPodHandler implement handling for the Pod hosting a minimal Kubernetes worker.
type workerPodHandler struct {
	// TODO: implement using kubemark or virtual kubelet.
	//  kubermark seems the best fit
	//  virtual kubelet with the mock provider seems a possible alternative, but I don't know if the mock providers has limitations that might limit usage.
	//  virtual kubelet with other providers seems overkill in this phase

	// TODO: in a follow up iteration we want to make it possible to store those objects in a dedicate ns on a separated cluster
	//  this brings in the limitation that objects for two clusters with the same name cannot be hosted in a single namespace as well as the need to rethink owner references.
	client client.Client

	controlPlaneEndpoint *vcsimv1.ControlPlaneEndpoint
	cluster              *clusterv1beta1.Cluster
	virtualMachine       client.Object

	overrideGetManagerContainer func(ctx context.Context) (*corev1.Container, error)
}

func (h *workerPodHandler) GetPods(ctx context.Context) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	if err := h.client.Get(ctx, client.ObjectKey{
		Namespace: h.virtualMachine.GetNamespace(),
		Name:      h.virtualMachine.GetName(),
	}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get pod")
	}
	return pod, nil
}

func (h *workerPodHandler) Generate(ctx context.Context, kubernetesVersion string) error {
	managerContainerFunc := h.getManagerContainer
	if h.overrideGetManagerContainer != nil {
		managerContainerFunc = h.overrideGetManagerContainer
	}
	managerContainer, err := managerContainerFunc(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get manager container")
	}

	semVer, err := semver.ParseTolerant(kubernetesVersion)
	if err != nil {
		return errors.Wrap(err, "failed to parse DockerMachine version")
	}
	kindMapping := kind.GetMapping(semVer, "")
	image := kindMapping.Image

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.virtualMachine.GetNamespace(),
			Name:      h.virtualMachine.GetName(),
			Labels: map[string]string{
				// Useful labels
				clusterv1beta1.ClusterNameLabel: h.cluster.Name,
			},
		},
		Spec: corev1.PodSpec{
			// make hostname match container name
			Hostname: h.virtualMachine.GetName(),
			InitContainers: []corev1.Container{
				// Use an init container to generate all the key, certificates and KubeConfig files
				// required for the worker to run.
				generateWorkerFilesContainer(managerContainer.Image, h.cluster.Name, h.cluster.Spec.ControlPlaneEndpoint.Host),
			},
			Containers: []corev1.Container{
				{
					Name:  "kind-node",
					Image: image,
					VolumeMounts: []corev1.VolumeMount{
						{ // various things depend on working /tmp
							Name:      "tmp",
							MountPath: "/tmp",
						},
						{ // various things depend on working /tmp
							Name:      "run",
							MountPath: "/run",
						},
						{ // containerd want to write files in /sys/fs/cgroup
							Name:      "sys-fs-cgroup",
							MountPath: "/sys/fs/cgroup",
						},
						{ // kubelet want to write files in /var/lib/kubelet/
							Name:      "var-lib-kubelet",
							MountPath: "/var/lib/kubelet",
						},
						{ // some k8s things want to read /lib/modules
							Name:      "lib-modules",
							MountPath: "/lib/modules",
							ReadOnly:  true,
						},
						{
							Name:      "etc-kubernetes",
							MountPath: "/etc/kubernetes",
						},
						{
							Name:      "kubelet-service",
							MountPath: "/etc/systemd/system/kubelet.service.d",
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.BoolPtr(true),
					},
				},
			},
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: "RuntimeDefault",
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
			Volumes: []corev1.Volume{
				{ // various things depend on working /tmp
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
				{ // systemd wants a writable /run
					Name: "run",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
				{ // containerd want to write files in /sys/fs/cgroup  // FIXME: it is failing
					Name: "sys-fs-cgroup",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: " /sys/fs/cgroup",
							Type: ptr.To(corev1.HostPathDirectory),
						},
					},
				},
				{ // kubelet want to write files in /var/lib/kubelet/
					Name: "var-lib-kubelet",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
				{ // some k8s things want to read /lib/modules
					Name: "lib-modules",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/lib/modules",
						},
					},
				},
				{
					Name: "etc-kubernetes",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "kubelet-service",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	if err := h.client.Create(ctx, pod); err != nil {
		return errors.Wrap(err, "failed to create control plane pod")
	}

	// Wait for the pod to show up in the cache
	if err := wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := h.client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
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

func (h *workerPodHandler) getManagerContainer(ctx context.Context) (*corev1.Container, error) {
	// Gets info about the Pod is running the manager in.
	managerPodNamespace := os.Getenv("POD_NAMESPACE")
	managerPodName := os.Getenv("POD_NAME")
	managerPodUID := types.UID(os.Getenv("POD_UID"))

	// Gets the Pod is running the manager in from the management cluster and validate it is the right one.
	managerPod := &corev1.Pod{}
	managerPodKey := types.NamespacedName{Namespace: managerPodNamespace, Name: managerPodName}
	if err := h.client.Get(ctx, managerPodKey, managerPod); err != nil {
		return nil, errors.Wrap(err, "failed to get manager pod")
	}
	if managerPod.UID != managerPodUID {
		return nil, errors.Errorf("manager pod UID does not match, expected %s, got %s", managerPodUID, managerPod.UID)
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
		return nil, errors.New("failed to get container from manager pod")
	}
	return managerContainer, nil
}

func (h *workerPodHandler) Delete(ctx context.Context, podName string) error {

	return nil
}

func generateWorkerFilesContainer(managerImage string, clusterName string, controlPaneEndPointHost string) corev1.Container {
	c := corev1.Container{
		Name: "generate-files",
		// Note: we are using the manager instead of another binary for convenience (the manager is already built and packaged
		// into an image that is published during the release process).
		Image:           managerImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/manager",
			"--generate-worker-virtual-machine-kubernetes-backend-files",
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
				Name:      "var-lib-kubelet",
				MountPath: "/var/lib/kubelet",
			},
			{
				Name:      "kubelet-service",
				MountPath: "/etc/systemd/system/kubelet.service.d",
			},
			{
				Name:      "etc-kubernetes",
				MountPath: "/etc/kubernetes",
			},
		},
	}
	return c
}
