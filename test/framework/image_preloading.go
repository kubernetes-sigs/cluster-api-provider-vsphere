/*
Copyright 2024 The Kubernetes Authors.

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

package framework

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/httpstream" //nolint:staticcheck // Let's migrate this to the new package after we did the same in core Cluster API.
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func LoadImagesFunc(ctx context.Context) func(clusterProxy framework.ClusterProxy) {
	sourceFile := os.Getenv("DOCKER_IMAGE_TAR")
	Expect(sourceFile).ToNot(BeEmpty(), "DOCKER_IMAGE_TAR must be set")

	return func(clusterProxy framework.ClusterProxy) {
		loadImagesToCluster(ctx, sourceFile, clusterProxy)
	}
}

// loadImagesToCluster deploys a privileged daemonset and uses it to stream-load container images.
func loadImagesToCluster(ctx context.Context, sourceFile string, clusterProxy framework.ClusterProxy) {
	daemonSet, daemonSetMutateFn, daemonSetLabels := getPreloadDaemonset()
	ctrlClient := clusterProxy.GetClient()

	// Create the DaemonSet.
	_, err := controllerutil.CreateOrPatch(ctx, ctrlClient, daemonSet, daemonSetMutateFn)
	Expect(err).ToNot(HaveOccurred())

	// Create informer.
	podInformer, err := clusterProxy.GetCache(ctx).GetInformer(ctx, &corev1.Pod{})
	Expect(err).ToNot(HaveOccurred(), "Failed to create controller-runtime informer from cache")

	// Add event handler to informer
	eventHandler := newLoadImagesEventHandler(ctx, clusterProxy, sourceFile, labels.Set(daemonSetLabels).AsSelector())
	handlerRegistration, err := podInformer.AddEventHandler(eventHandler)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		<-ctx.Done()
		Expect(podInformer.RemoveEventHandler(handlerRegistration)).To(Succeed())
	}()
}

type loadImagesEventHandler struct {
	//nolint:containedctx
	ctx           context.Context
	clusterProxy  framework.ClusterProxy
	sourceFile    string
	selector      labels.Selector
	imageLoadPods sync.Map
}

func newLoadImagesEventHandler(ctx context.Context, clusterProxy framework.ClusterProxy, sourceFile string, selector labels.Selector) cache.ResourceEventHandler {
	return &loadImagesEventHandler{
		ctx:           ctx,
		clusterProxy:  clusterProxy,
		sourceFile:    sourceFile,
		selector:      selector,
		imageLoadPods: sync.Map{},
	}
}

func (eh *loadImagesEventHandler) OnAdd(obj interface{}, _ bool) {
	pod := obj.(*corev1.Pod)

	eh.loadImagesViaPod(eh.ctx, eh.clusterProxy, eh.sourceFile, pod, pod.Spec.Containers[0].Name)
}

func (eh *loadImagesEventHandler) OnUpdate(_, newObj interface{}) {
	pod := newObj.(*corev1.Pod)
	eh.loadImagesViaPod(eh.ctx, eh.clusterProxy, eh.sourceFile, pod, pod.Spec.Containers[0].Name)
}

func (eh *loadImagesEventHandler) OnDelete(_ interface{}) {}

func (eh *loadImagesEventHandler) loadImagesViaPod(ctx context.Context, clusterProxy framework.ClusterProxy, sourceFile string, pod *corev1.Pod, containerName string) {
	if pod.Namespace != metav1.NamespaceSystem { // The DaemonSet is deployed in kube-system.
		return
	}
	if !eh.selector.Matches(labels.Set(pod.GetLabels())) {
		return
	}
	if pod.Status.Phase != corev1.PodRunning {
		return
	}
	if _, loaded := eh.imageLoadPods.LoadOrStore(pod.GetUID(), struct{}{}); loaded {
		return
	}

	Byf("Loading images to Node %s via Pod %s", pod.Spec.NodeName, klog.KObj(pod))

	// Open source tar file.
	reader, writer := io.Pipe()
	file, err := os.Open(filepath.Clean(sourceFile))
	if err != nil {
		Byf("Failed loading images to Node %s via Pod %s: failed to load source file %s: %v", pod.Spec.NodeName, klog.KObj(pod), sourceFile, err)
		return
	}

	// Use go routine to pipe source file content into then stdin.
	go func(file *os.File, writer io.WriteCloser) {
		defer writer.Close()
		defer file.Close()
		// Ignoring the error here because the execPod command should fail in case of
		// failure copying over the data.
		_, err := io.Copy(writer, file)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Failed to copy file data to io.Pipe: %v\n", err)
		}
	}(file, writer)

	// Load the container images using ctr and delete the file.
	loadCommand := "ctr -n k8s.io images import -"

	if err := execPod(ctx, clusterProxy, pod.Namespace, pod.Name, containerName, loadCommand, reader); err != nil {
		Byf("Failed loading images to Node %s via Pod %s: %s", pod.Spec.NodeName, klog.KObj(pod), err)
		eh.imageLoadPods.Delete(pod.GetUID()) // Delete so we try again on next update.
		return
	}
	Byf("Succeeded loading images to Node %s via Pod %s", pod.Spec.NodeName, klog.KObj(pod))
}

// execPod executes a command at a pod.
// xref: https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubectl/pkg/cmd/exec/exec.go#L123
func execPod(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, podName, containerName, cmd string, stdin io.Reader) error {
	var hasStdin bool
	if stdin != nil {
		hasStdin = true
	}

	req := clusterProxy.GetClientSet().CoreV1().RESTClient().Post().
		Namespace(namespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"/bin/sh", "-c", cmd},
			Stdin:     hasStdin,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clusterProxy.GetRESTConfig(), "POST", req.URL())
	if err != nil {
		return err
	}
	// WebSocketExecutor must be "GET" method as described in RFC 6455 Sec. 4.1 (page 17).
	websocketExec, err := remotecommand.NewWebSocketExecutor(clusterProxy.GetRESTConfig(), "GET", req.URL().String())
	if err != nil {
		return err
	}
	exec, err = remotecommand.NewFallbackExecutor(websocketExec, exec, httpstream.IsUpgradeFailure)
	if err != nil {
		return err
	}

	var stdout, stderr bytes.Buffer

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return errors.Wrapf(err, "running command %q stdout=%q, stderr=%q", cmd, stdout.String(), stderr.String())
	}

	return nil
}

func getPreloadDaemonset() (*appsv1.DaemonSet, controllerutil.MutateFn, map[string]string) {
	labels := map[string]string{
		"app": "image-preloader",
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "image-preloader",
			Labels:    labels,
		},
	}
	mutateFunc := func() error {
		ds.Labels = labels
		ds.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "importer",
							Image: "gcr.io/k8s-staging-capi-vsphere/extra/ctr:latest",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "containerd-socket",
									MountPath: "/run/containerd/containerd.sock",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "containerd-socket",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/containerd/containerd.sock",
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						// Tolerate any taint.
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
		}
		return nil
	}
	return ds, mutateFunc, labels
}
