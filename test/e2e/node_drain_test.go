/*
Copyright 2020 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	. "sigs.k8s.io/cluster-api/test/framework/ginkgoextensions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const boskosResourceLabel = "capv-e2e-test-boskos-resource"

var _ = Describe("When testing Node drain [supervisor]", func() {
	const specName = "node-drain" // copied from CAPI
	Setup(specName, func(testSpecificSettingsGetter func() testSettings) {
		capi_e2e.NodeDrainTimeoutSpec(ctx, func() capi_e2e.NodeDrainTimeoutSpecInput {
			return capi_e2e.NodeDrainTimeoutSpecInput{
				E2EConfig:              e2eConfig,
				ClusterctlConfigPath:   testSpecificSettingsGetter().ClusterctlConfigPath,
				BootstrapClusterProxy:  bootstrapClusterProxy,
				ArtifactFolder:         artifactFolder,
				SkipCleanup:            skipCleanup,
				Flavor:                 ptr.To(testSpecificSettingsGetter().FlavorForMode("topology")),
				InfrastructureProvider: ptr.To("vsphere"),
				PostNamespaceCreated:   testSpecificSettingsGetter().PostNamespaceCreatedFunc,
				// Add verification for CSI blocking volume detachments.
				VerifyNodeVolumeDetach: true,
				CreateAdditionalResources: func(ctx context.Context, clusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster) {
					// Add a MachineDrainRule to ensure kube-system pods get evicted first and don't mess up the condition assertions.
					deployKubeSystemMachineDrainRule(ctx, clusterProxy, cluster)
					// Add a statefulset which uses CSI.
					deployStatefulSetAndBlockCSI(ctx, clusterProxy, cluster)
				},
				UnblockNodeVolumeDetachment: unblockNodeVolumeDetachment,
			}
		})
	})
})

func deployKubeSystemMachineDrainRule(ctx context.Context, clusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster) {
	mdRule := &clusterv1.MachineDrainRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kube-system", cluster.Name),
			Namespace: cluster.Namespace,
		},
		Spec: clusterv1.MachineDrainRuleSpec{
			Drain: clusterv1.MachineDrainRuleDrainConfig{
				Behavior: clusterv1.MachineDrainRuleDrainBehaviorDrain,
				Order:    ptr.To[int32](-20),
			},
			Machines: []clusterv1.MachineDrainRuleMachineSelector{
				// Select all Machines with the ClusterNameLabel belonging to Clusters with the ClusterNameLabel.
				{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							clusterv1.ClusterNameLabel: cluster.Name,
						},
					},
					ClusterSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							clusterv1.ClusterNameLabel: cluster.Name,
						},
					},
				},
			},
			Pods: []clusterv1.MachineDrainRulePodSelector{
				// Select all Pods in namespace "kube-system".
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "kubernetes.io/metadata.name",
								Operator: metav1.LabelSelectorOpIn,
								Values: []string{
									metav1.NamespaceSystem,
								},
							},
						},
					},
				},
			},
		},
	}

	Expect(clusterProxy.GetClient().Create(ctx, mdRule)).To(Succeed())
}

func deployStatefulSetAndBlockCSI(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster) {
	controlplane := framework.DiscoveryAndWaitForControlPlaneInitialized(ctx, framework.DiscoveryAndWaitForControlPlaneInitializedInput{
		Lister:  bootstrapClusterProxy.GetClient(),
		Cluster: cluster,
	}, e2eConfig.GetIntervals("node-drain", "wait-control-plane")...)

	mds := framework.DiscoveryAndWaitForMachineDeployments(ctx, framework.DiscoveryAndWaitForMachineDeploymentsInput{
		Lister:  bootstrapClusterProxy.GetClient(),
		Cluster: cluster,
	}, e2eConfig.GetIntervals("node-drain", "wait-worker-nodes")...)

	// This label will be added to all Machines so we can later create Pods on the right Nodes.
	nodeOwnerLabelKey := "owner.node.cluster.x-k8s.io"

	workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name)

	By("Deploy a storageclass for CSI")
	err := workloadClusterProxy.GetClient().Create(ctx, &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sts-pvc",
		},
		Provisioner:   "csi.vsphere.vmware.com",
		ReclaimPolicy: ptr.To(corev1.PersistentVolumeReclaimDelete),
	})
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	By("Deploy StatefulSets with evictable Pods without finalizer on control plane and MachineDeployment Nodes.")
	deployEvictablePod(ctx, deployEvictablePodInput{
		WorkloadClusterProxy: workloadClusterProxy,
		ControlPlane:         controlplane,
		StatefulSetName:      "sts-cp",
		// The delete condition lists objects in status by alphabetical order. these pods are expected to get removed last so
		// this namespace makes the pods to be listed after the ones used in capi (`evictable-workoad` / `unevictable-workload`)
		// so the regex matching wokrs out.
		Namespace:                           "volume-evictable-workload",
		NodeSelector:                        map[string]string{nodeOwnerLabelKey: "KubeadmControlPlane-" + controlplane.Name},
		WaitForStatefulSetAvailableInterval: e2eConfig.GetIntervals("node-drain", "wait-statefulset-available"),
	})
	for _, md := range mds {
		deployEvictablePod(ctx, deployEvictablePodInput{
			WorkloadClusterProxy: workloadClusterProxy,
			MachineDeployment:    md,
			StatefulSetName:      fmt.Sprintf("sts-%s", md.Name),
			// The delete condition lists objects in status by alphabetical order. these pods are expected to get removed last so
			// this namespace makes the pods to be listed after the ones used in capi (`evictable-workoad` / `unevictable-workload`)
			// so the regex matching wokrs out.
			Namespace:                           "volume-evictable-workload",
			NodeSelector:                        map[string]string{nodeOwnerLabelKey: "MachineDeployment-" + md.Name},
			WaitForStatefulSetAvailableInterval: e2eConfig.GetIntervals("node-drain", "wait-statefulset-available"),
		})
	}

	By("Scaling down the CSI controller to block lifecycle of PVC's")
	csiController := &appsv1.Deployment{}
	csiControllerKey := client.ObjectKey{
		Namespace: "vmware-system-csi",
		Name:      "vsphere-csi-controller",
	}
	Expect(workloadClusterProxy.GetClient().Get(ctx, csiControllerKey, csiController)).To(Succeed())
	patchHelper, err := patch.NewHelper(csiController, workloadClusterProxy.GetClient())
	Expect(err).ToNot(HaveOccurred())
	csiController.Spec.Replicas = ptr.To[int32](0)
	Expect(patchHelper.Patch(ctx, csiController)).To(Succeed())
	waitForDeploymentScaledDown(ctx, workloadClusterProxy.GetClient(), csiControllerKey, e2eConfig.GetIntervals("node-drain", "wait-deployment-available")...)
}

func unblockNodeVolumeDetachment(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, cluster *clusterv1.Cluster) {
	workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name)

	By("Scaling up the CSI controller to unblock lifecycle of PVC's")
	csiController := &appsv1.Deployment{}
	csiControllerKey := client.ObjectKey{
		Namespace: "vmware-system-csi",
		Name:      "vsphere-csi-controller",
	}
	Expect(workloadClusterProxy.GetClient().Get(ctx, csiControllerKey, csiController)).To(Succeed())
	patchHelper, err := patch.NewHelper(csiController, workloadClusterProxy.GetClient())
	Expect(err).ToNot(HaveOccurred())
	csiController.Spec.Replicas = ptr.To[int32](1)
	Expect(patchHelper.Patch(ctx, csiController)).To(Succeed())

	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     workloadClusterProxy.GetClient(),
		Deployment: csiController,
	}, e2eConfig.GetIntervals("node-drain", "wait-deployment-available")...)
}

func waitForDeploymentScaledDown(ctx context.Context, getter framework.Getter, objectKey client.ObjectKey, intervals ...interface{}) {
	Byf("Waiting for deployment %s to be scaled to 0", objectKey)
	deployment := &appsv1.Deployment{}
	Eventually(func() bool {
		if err := getter.Get(ctx, objectKey, deployment); err != nil {
			return false
		}
		if deployment.Status.Replicas == 0 && deployment.Status.AvailableReplicas == 0 {
			return true
		}
		return false
	}, intervals...).Should(BeTrue())
}

const (
	nodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
)

type deployEvictablePodInput struct {
	WorkloadClusterProxy framework.ClusterProxy
	ControlPlane         *controlplanev1.KubeadmControlPlane
	MachineDeployment    *clusterv1.MachineDeployment
	StatefulSetName      string
	Namespace            string
	NodeSelector         map[string]string

	ModifyStatefulSet func(statefulSet *appsv1.StatefulSet)

	WaitForStatefulSetAvailableInterval []interface{}
}

// deployEvictablePod will deploy a StatefulSet on a ControlPlane or MachineDeployment.
// It will deploy one Pod replica to each Machine.
func deployEvictablePod(ctx context.Context, input deployEvictablePodInput) {
	Expect(input.StatefulSetName).ToNot(BeNil(), "Need a statefulset name in DeployUnevictablePod")
	Expect(input.Namespace).ToNot(BeNil(), "Need a namespace in DeployUnevictablePod")
	Expect(input.WorkloadClusterProxy).ToNot(BeNil(), "Need a workloadClusterProxy in DeployUnevictablePod")
	Expect((input.MachineDeployment == nil && input.ControlPlane != nil) ||
		(input.MachineDeployment != nil && input.ControlPlane == nil)).To(BeTrue(), "Either MachineDeployment or ControlPlane must be set in DeployUnevictablePod")

	framework.EnsureNamespace(ctx, input.WorkloadClusterProxy.GetClient(), input.Namespace)

	workloadStatefulSet := generateStatefulset(generateStatefulsetInput{
		ControlPlane:      input.ControlPlane,
		MachineDeployment: input.MachineDeployment,
		Name:              input.StatefulSetName,
		Namespace:         input.Namespace,
		NodeSelector:      input.NodeSelector,
	})

	if input.ModifyStatefulSet != nil {
		input.ModifyStatefulSet(workloadStatefulSet)
	}

	workloadClient := input.WorkloadClusterProxy.GetClientSet()

	addStatefulSetToWorkloadCluster(ctx, addStatefulSetToWorkloadClusterInput{
		Namespace:   input.Namespace,
		ClientSet:   workloadClient,
		StatefulSet: workloadStatefulSet,
	})

	waitForStatefulSetsAvailable(ctx, WaitForStatefulSetsAvailableInput{
		Getter:      input.WorkloadClusterProxy.GetClient(),
		StatefulSet: workloadStatefulSet,
	}, input.WaitForStatefulSetAvailableInterval...)
}

type generateStatefulsetInput struct {
	ControlPlane      *controlplanev1.KubeadmControlPlane
	MachineDeployment *clusterv1.MachineDeployment
	Name              string
	Namespace         string
	NodeSelector      map[string]string
}

func generateStatefulset(input generateStatefulsetInput) *appsv1.StatefulSet {
	workloadStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":         "nonstop",
					"statefulset": input.Name,
					"e2e-test":    "node-drain",
					// All labels get propagated down to CNS Volumes in vSphere.
					// This label will be used by the janitor to cleanup orphaned CNS volumes.
					boskosResourceLabel: os.Getenv("BOSKOS_RESOURCE_NAME"),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":         "nonstop",
						"statefulset": input.Name,
						"e2e-test":    "node-drain",
						// All labels get propagated down to CNS Volumes in vSphere.
						// This label will be used by the janitor to cleanup orphaned CNS volumes.
						boskosResourceLabel: os.Getenv("BOSKOS_RESOURCE_NAME"),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "main",
						Image: "registry.k8s.io/pause:3.10",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "sts-pvc",
							MountPath: "/data",
						}},
					}},
					Affinity: &corev1.Affinity{
						// Make sure only 1 Pod of this StatefulSet can run on the same Node.
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{{
											Key:      "statefulset",
											Operator: "In",
											Values:   []string{input.Name},
										}},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sts-pvc",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
					StorageClassName: ptr.To("sts-pvc"),
				},
			}},
		},
	}

	if input.ControlPlane != nil {
		workloadStatefulSet.Spec.Template.Spec.NodeSelector = map[string]string{nodeRoleControlPlane: ""}
		workloadStatefulSet.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			{
				Key:    nodeRoleControlPlane,
				Effect: "NoSchedule",
			},
		}
		workloadStatefulSet.Spec.Replicas = input.ControlPlane.Spec.Replicas
	}
	if input.MachineDeployment != nil {
		workloadStatefulSet.Spec.Replicas = input.MachineDeployment.Spec.Replicas
	}

	// Note: If set, the NodeSelector field overwrites the NodeSelector we set above for control plane nodes.
	if input.NodeSelector != nil {
		workloadStatefulSet.Spec.Template.Spec.NodeSelector = input.NodeSelector
	}

	return workloadStatefulSet
}

type addStatefulSetToWorkloadClusterInput struct {
	ClientSet   *kubernetes.Clientset
	StatefulSet *appsv1.StatefulSet
	Namespace   string
}

func addStatefulSetToWorkloadCluster(ctx context.Context, input addStatefulSetToWorkloadClusterInput) {
	Eventually(func() error {
		result, err := input.ClientSet.AppsV1().StatefulSets(input.Namespace).Create(ctx, input.StatefulSet, metav1.CreateOptions{})
		if result != nil && err == nil {
			return nil
		}
		return fmt.Errorf("statefulset %s not successfully created in workload cluster: %v", klog.KObj(input.StatefulSet), err)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to create statefulset %s in workload cluster", klog.KObj(input.StatefulSet))
}

const (
	retryableOperationInterval = 3 * time.Second
	// retryableOperationTimeout requires a higher value especially for self-hosted upgrades.
	// Short unavailability of the Kube APIServer due to joining etcd members paired with unreachable conversion webhooks due to
	// failed leader election and thus controller restarts lead to longer taking retries.
	// The timeout occurs when listing machines in `GetControlPlaneMachinesByCluster`.
	retryableOperationTimeout = 3 * time.Minute
)

// WaitForStatefulSetsAvailableInput is the input for WaitForStatefulSetsAvailable.
type WaitForStatefulSetsAvailableInput struct {
	Getter      framework.Getter
	StatefulSet *appsv1.StatefulSet
}

// waitForStatefulSetsAvailable waits until the StatefulSet has status.Available = True, that signals that
// all the desired replicas are in place.
// This can be used to check if Cluster API controllers installed in the management cluster are working.
func waitForStatefulSetsAvailable(ctx context.Context, input WaitForStatefulSetsAvailableInput, intervals ...interface{}) {
	Byf("Waiting for statefulset %s to be available", klog.KObj(input.StatefulSet))
	statefulSet := &appsv1.StatefulSet{}
	Eventually(func() bool {
		key := client.ObjectKey{
			Namespace: input.StatefulSet.GetNamespace(),
			Name:      input.StatefulSet.GetName(),
		}
		if err := input.Getter.Get(ctx, key, statefulSet); err != nil {
			return false
		}
		if *statefulSet.Spec.Replicas == statefulSet.Status.AvailableReplicas {
			return true
		}
		return false
	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedStatefulSet(input, statefulSet) })
}

// DescribeFailedStatefulSet returns detailed output to help debug a statefulSet failure in e2e.
func DescribeFailedStatefulSet(input WaitForStatefulSetsAvailableInput, statefulSet *appsv1.StatefulSet) string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("StatefulSet %s failed to get status.Available = True condition",
		klog.KObj(input.StatefulSet)))
	if statefulSet == nil {
		b.WriteString("\nStatefulSet: nil\n")
	} else {
		b.WriteString(fmt.Sprintf("\nStatefulSet:\n%s\n", framework.PrettyPrint(statefulSet)))
	}
	return b.String()
}
