/*
Copyright The Kubernetes Authors.

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

// main is the main package for cleanup-scale-env.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	vmoprv1alpha5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmwarev1 "sigs.k8s.io/cluster-api-provider-vsphere/api/supervisor/v1beta2"
)

const (
	concurrencyLimit = 100
)

var (
	shouldCleanupObjects        = flag.Bool("cleanup-objects", true, "")
	shouldCleanupPrometheusData = flag.Bool("cleanup-prometheus-data", true, "")
	shouldCleanupLokiData       = flag.Bool("cleanup-loki-data", true, "")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	restConfig := ctrl.GetConfigOrDie()

	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		log.Fatalf("Failed to get REST config: %v", err)
	}

	fmt.Printf("Please confirm via [y/n] that you want to run cleanup on %s with:\n* cleanup-objects: %t\n* cleanup-prometheus-data: %t\n* cleanup-loki-data: %t\n", restConfig.Host, *shouldCleanupObjects, *shouldCleanupPrometheusData, *shouldCleanupLokiData)
	response, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Fatalln("Error reading input. Please try again.")
	}
	response = strings.ToLower(strings.TrimSpace(response))
	if response != "y" && response != "yes" {
		log.Println("Aborting.")
		os.Exit(0)
	}

	if *shouldCleanupObjects {
		if err := cleanupObjects(ctx, c); err != nil {
			log.Fatalf("Failed to cleanup objects: %v", err)
		}
	}

	if *shouldCleanupPrometheusData {
		if err := cleanupPrometheusData(ctx, c); err != nil {
			log.Fatalf("Failed to cleanup Prometheus data: %v", err)
		}
	}

	if *shouldCleanupLokiData {
		if err := cleanupLokiData(ctx, c); err != nil {
			log.Fatalf("Failed to cleanup Loki data: %v", err)
		}
	}
}

func cleanupObjects(ctx context.Context, c client.Client) error {
	log.Println("Deleting scale- namespaces")
	namespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, namespaces); err != nil {
		return err
	}
	namespacesToDelete := slices.DeleteFunc(namespaces.Items, func(namespace corev1.Namespace) bool {
		return !strings.HasPrefix(namespace.Name, "scale-") || !namespace.DeletionTimestamp.IsZero()
	})

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrencyLimit)
	for i, ns := range namespacesToDelete {
		wg.Add(1)
		sem <- struct{}{} // Acquire a token. This blocks if the channel is full (i.e. the concurrencyLimit is reached).

		go func(i int, obj *corev1.Namespace) {
			defer func() { <-sem }() // Release the token when the goroutine finishes
			defer wg.Done()

			if i != 0 && i%1000 == 0 {
				log.Printf("Deleted first %d Namespace objects", i)
			}
			if err := c.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
				log.Printf("Error deleting Namespace %s: %v\n", klog.KObj(obj), err)
			}
		}(i, &ns)
	}
	wg.Wait()

	gvksToDelete := []schema.GroupVersionKind{
		vmoprv1alpha5.GroupVersion.WithKind("VirtualMachine"),
		vmwarev1.GroupVersion.WithKind("VSphereMachine"),
		clusterv1.GroupVersion.WithKind("Machine"),
		clusterv1.GroupVersion.WithKind("MachineSet"),
		clusterv1.GroupVersion.WithKind("MachineDeployment"),
		controlplanev1.GroupVersion.WithKind("KubeadmControlPlane"),
		vmwarev1.GroupVersion.WithKind("VSphereCluster"),
		clusterv1.GroupVersion.WithKind("Cluster"),
		// Main Go module cannot have a dependency to the ./test Go module.
		{Group: "vcsim.infrastructure.cluster.x-k8s.io", Version: "v1alpha1", Kind: "ControlPlaneEndpoint"},
		{Group: "vcsim.infrastructure.cluster.x-k8s.io", Version: "v1alpha1", Kind: "EnvVar"},
	}

	objectsStillExist := true
	for objectsStillExist {
		objectsStillExist = false

		for _, gvk := range gvksToDelete {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind + "List",
			})

			if err := c.List(ctx, list); err != nil {
				objectsStillExist = true
				log.Printf("Error listing %s: %v\n", gvk.Kind, err)
				continue
			}

			if len(list.Items) == 0 {
				continue
			}

			objectsStillExist = true
			log.Printf("Deleting %s objects (%d still exist)", gvk.Kind, len(list.Items))

			objectsToDelete := slices.DeleteFunc(list.Items, func(u unstructured.Unstructured) bool {
				return !u.GetDeletionTimestamp().IsZero()
			})

			var wg sync.WaitGroup
			sem := make(chan struct{}, concurrencyLimit)
			for i, obj := range objectsToDelete {
				wg.Add(1)
				sem <- struct{}{} // Acquire a token. This blocks if the channel is full (i.e. the concurrencyLimit is reached).

				go func(i int, obj *unstructured.Unstructured) {
					defer func() { <-sem }() // Release the token when the goroutine finishes
					defer wg.Done()

					if i != 0 && i%1000 == 0 {
						log.Printf("Deleted first %d %s objects", i, gvk.Kind)
					}
					if err := c.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
						log.Printf("Error deleting %s %s: %v\n", gvk.Kind, klog.KObj(obj), err)
					}
				}(i, &obj)
			}
			wg.Wait()

			// Instantiate a new empty list (reusing the list from above does not work).
			list = &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind + "List",
			})
			if err := c.List(ctx, list); err != nil {
				log.Printf("Error listing %s: %v\n", gvk.Kind, err)
				continue
			}

			objectsToRemoveFinalizer := slices.DeleteFunc(list.Items, func(u unstructured.Unstructured) bool {
				return len(u.GetFinalizers()) == 0
			})

			sem = make(chan struct{}, concurrencyLimit)
			for i, obj := range objectsToRemoveFinalizer {
				wg.Add(1)
				sem <- struct{}{} // Acquire a token. This blocks if the channel is full (i.e. the concurrencyLimit is reached).

				go func(i int, obj *unstructured.Unstructured) {
					defer func() { <-sem }() // Release the token when the goroutine finishes
					defer wg.Done()

					if i != 0 && i%1000 == 0 {
						log.Printf("Removed finalizers from first %d %s objects", i, gvk.Kind)
					}
					origObj := obj.DeepCopy()
					obj.SetFinalizers(nil)
					if err := c.Patch(ctx, obj, client.MergeFrom(origObj)); err != nil && !apierrors.IsNotFound(err) {
						log.Printf("Error removing finalizers from %s %s: %v\n", gvk.Kind, klog.KObj(obj), err)
					}
				}(i, &obj)
			}
			wg.Wait()
		}

		if objectsStillExist {
			time.Sleep(5 * time.Second) // Give Kubernetes and specifically the garbage collector some time to make progress.
		}
	}

	log.Println("Deleting scale namespace")
	if err := c.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "scale"}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	podList := &corev1.PodList{}
	if err := c.List(ctx, podList,
		client.InNamespace("vcsim-system"),
		client.MatchingLabels{
			"cluster.x-k8s.io/provider": "runtime-extension-vcsim",
		},
	); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		log.Printf("Deleting Pod %s", klog.KObj(&pod))
		if err := c.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	for {
		namespaces = &corev1.NamespaceList{}
		if err := c.List(ctx, namespaces); err != nil {
			log.Printf("Error listing Namespace: %v\n", err)
			continue
		}
		namespacesRemaining := slices.DeleteFunc(namespaces.Items, func(namespace corev1.Namespace) bool {
			return !strings.HasPrefix(namespace.Name, "scale")
		})

		if len(namespacesRemaining) == 0 {
			break
		}

		log.Printf("There are still %d scale namespaces that should be gone\n", len(namespacesRemaining))

		time.Sleep(5 * time.Second) // Give Kubernetes and specifically the garbage collector some time to make progress.
	}

	log.Println("Object cleanup completed")
	return nil
}

func cleanupPrometheusData(ctx context.Context, c client.Client) error {
	log.Println("Deleting Prometheus PVC")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "observability",
			Name:      "prometheus-server",
		},
	}
	if err := c.Delete(ctx, pvc); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	podList := &corev1.PodList{}
	if err := c.List(ctx, podList,
		client.InNamespace("observability"),
		client.MatchingLabels{
			"app.kubernetes.io/name": "prometheus",
		},
	); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		log.Printf("Deleting Pod %s", klog.KObj(&pod))
		if err := c.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Wait for PVC to be gone.
	if err := wait.ExponentialBackoff(wait.Backoff{
		Steps:    10,
		Duration: 1 * time.Second,
		Factor:   1.0,
	}, func() (done bool, err error) {
		if err := c.Get(ctx, client.ObjectKey{Namespace: "observability", Name: "prometheus-server"}, &corev1.PersistentVolumeClaim{}); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			log.Printf("Error occurred when getting PersistentVolumeClaim %s: %s\n", klog.KObj(pvc), err)
		}
		return false, nil
	}); err != nil {
		return err
	}

	log.Println("Creating new Prometheus PVC")
	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "observability",
			Name:      "prometheus-server",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					"storage": func() resource.Quantity {
						q, _ := resource.ParseQuantity("256Gi")
						return q
					}(),
				},
			},
		},
	}
	return c.Create(ctx, pvc)
}

func cleanupLokiData(ctx context.Context, c client.Client) error {
	log.Println("Deleting Loki PVC")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "observability",
			Name:      "storage-loki-0",
		},
	}
	if err := c.Delete(ctx, pvc); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	podList := &corev1.PodList{}
	if err := c.List(ctx, podList,
		client.InNamespace("observability"),
		client.MatchingLabels{
			"app.kubernetes.io/name": "loki",
		},
	); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		log.Printf("Deleting Pod %s", klog.KObj(&pod))
		if err := c.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Note: The Loki PVC is automatically re-created.

	return nil
}
