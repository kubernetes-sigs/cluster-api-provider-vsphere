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

package vmoperator

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	vmoprv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha1"
	topologyv1 "github.com/vmware-tanzu/vm-operator/external/tanzu-topology/api/v1alpha1"
	"github.com/vmware/govmomi/pbm"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25/soap"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
	vcsimv1 "sigs.k8s.io/cluster-api-provider-vsphere/test/infrastructure/vcsim/api/v1alpha1"
)

const DefaultNamespace = "vmware-system-vmop"

const (
	// NOTE: ConfigMapName/ConfigMapKey values below must match what defined in pkg/vmprovider/providers/vsphere/config/config.go.

	configMapName                     = "vsphere.provider.config.vmoperator.vmware.com"
	hostConfigMapKey                  = "VcPNID" // vcenter host
	portConfigMapKey                  = "VcPort"
	credentialSecretNameConfigMapKey  = "VcCredsSecretName" //nolint:gosec
	datacenterConfigMapKey            = "Datacenter"
	resourcePoolConfigMapKey          = "ResourcePool"
	folderConfigMapKey                = "Folder"
	storageClassRequiredConfigMapKey  = "StorageClassRequired"
	useInventoryConfigMapKey          = "UseInventoryAsContentSource"
	insecureSkipTLSVerifyConfigMapKey = "InsecureSkipTLSVerify"

	// Additional ConfigMapKey we are adding to the vm-operator config map for sake of convenience (not supported in vm-operator).

	serverURLConfigMapKey            = "CAPV-TEST-Server"
	datacenterNameConfigMapKey       = "CAPV-TEST-DatacenterName"
	distributedPortGroupConfigMapKey = "CAPV-TEST-PortGroup"

	// Const for the VcCredsSecret (hard-coded in vm-operator).
	vmOperatorSecretName = "vsphere.provider.credentials.vmoperator.vmware.com"

	usernameSecretKey = "username"
	passwordSecretKey = "password"

	// Additional key we are adding to the VcCredsSecret for sake of convenience (not supported in vm-operator).

	thumbprintSecretKey = "CAPV-TEST-Thumbprint" //nolint:gosec
)

// ReconcileDependencies reconciles dependencies for the vm-operator.
// NOTE: This func is idempotent, it creates objects if missing otherwise it uses existing ones
// (this will allow e.g. to update images once and re-use for many test run).
func ReconcileDependencies(ctx context.Context, c client.Client, dependenciesConfig *vcsimv1.VMOperatorDependencies) error {
	var retryError error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling dependencies for the VMOperator Deployment")

	config := dependenciesConfig.DeepCopy()

	// If we are using a VCenterSimulator, read it build a config.Spec.VCenter for it (so the code below can assume Spec.VCenter is always set).
	// Also, add default storage and vm class for vcsim in not otherwise specified.
	if config.Spec.VCenterSimulatorRef != nil {
		vCenterSimulator := &vcsimv1.VCenterSimulator{}
		if err := c.Get(ctx, client.ObjectKey{
			Namespace: config.Spec.VCenterSimulatorRef.Namespace,
			Name:      config.Spec.VCenterSimulatorRef.Name,
		}, vCenterSimulator); err != nil {
			return errors.Wrapf(err, "failed to get vCenterSimulator %s", klog.KRef(config.Spec.VCenterSimulatorRef.Namespace, config.Spec.VCenterSimulatorRef.Name))
		}

		config.SetVCenterFromVCenterSimulator(vCenterSimulator)
	}

	// default the OperatorRef if not specified.
	if config.Spec.OperatorRef == nil {
		config.Spec.OperatorRef = &vcsimv1.VMOperatorRef{Namespace: DefaultNamespace}
	}

	// Get a Client to VCenter and get holds on the relevant objects that should already exist
	params := session.NewParams().
		WithServer(config.Spec.VCenter.ServerURL).
		WithDatacenter(config.Spec.VCenter.Datacenter).
		WithThumbprint(config.Spec.VCenter.Thumbprint).
		WithUserInfo(config.Spec.VCenter.Username, config.Spec.VCenter.Password)

	s, err := session.GetOrCreate(ctx, params)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to vCenter Server instance to read dependency references")
	}

	datacenter, err := s.Finder.Datacenter(ctx, config.Spec.VCenter.Datacenter)
	if err != nil {
		return errors.Wrapf(err, "failed to get datacenter %s", config.Spec.VCenter.Datacenter)
	}

	cluster, err := s.Finder.ClusterComputeResource(ctx, config.Spec.VCenter.Cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to get cluster %s", config.Spec.VCenter.Cluster)
	}

	folder, err := s.Finder.Folder(ctx, config.Spec.VCenter.Folder)
	if err != nil {
		return errors.Wrapf(err, "failed to get folder %s", config.Spec.VCenter.Folder)
	}

	resourcePool, err := s.Finder.ResourcePool(ctx, config.Spec.VCenter.ResourcePool)
	if err != nil {
		return errors.Wrapf(err, "failed to get resourcePool %s", config.Spec.VCenter.ResourcePool)
	}

	contentLibraryDatastore, err := s.Finder.Datastore(ctx, config.Spec.VCenter.ContentLibrary.Datastore)
	if err != nil {
		return errors.Wrapf(err, "failed to get contentLibraryDatastore %s", config.Spec.VCenter.ContentLibrary.Datastore)
	}

	pbmClient, err := pbm.NewClient(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "failed to get storage policy client")
	}

	// Create StorageClasses & bind them to the user namespace via a ResourceQuota
	// NOTE: vm-operator is using the ResourceQuota to figure out which StorageClass can be used from a namespace
	for _, sc := range config.Spec.StorageClasses {
		storagePolicyID, err := pbmClient.ProfileIDByName(ctx, sc.StoragePolicy)
		if err != nil {
			return errors.Wrapf(err, "failed to get storage policy profile %s", sc.StoragePolicy)
		}

		storageClass := &storagev1.StorageClass{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: sc.Name,
			},
			Provisioner: "kubernetes.io/vsphere-volume",
			Parameters: map[string]string{
				"storagePolicyID": storagePolicyID,
			},
		}

		_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			retryError = nil
			if err := c.Get(ctx, client.ObjectKeyFromObject(storageClass), storageClass); err != nil {
				if !apierrors.IsNotFound(err) {
					retryError = errors.Wrapf(err, "failed to get vm-operator StorageClass %s", storageClass.Name)
					return false, nil
				}
				if err := c.Create(ctx, storageClass); err != nil {
					retryError = errors.Wrapf(err, "failed to create vm-operator StorageClass %s", storageClass.Name)
					return false, nil
				}
				log.Info("Created vm-operator StorageClass", "StorageClass", klog.KObj(storageClass))
			}
			return true, nil
		})
		if retryError != nil {
			return retryError
		}

		// TODO: rethink about this, for now we are creating a ResourceQuota with the same name of the StorageClass, might be this is not ok when hooking into a real vCenter
		resourceQuota := &corev1.ResourceQuota{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sc.Name,
				Namespace: config.Namespace,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceName(fmt.Sprintf("%s.storageclass.storage.k8s.io/requests.storage", storageClass.Name)): resource.MustParse("1Gi"),
				},
			},
		}

		_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			retryError = nil
			if err := c.Get(ctx, client.ObjectKeyFromObject(resourceQuota), resourceQuota); err != nil {
				if !apierrors.IsNotFound(err) {
					retryError = errors.Wrapf(err, "failed to get vm-operator ResourceQuota %s", resourceQuota.Name)
					return false, nil
				}
				if err := c.Create(ctx, resourceQuota); err != nil {
					retryError = errors.Wrapf(err, "failed to create vm-operator ResourceQuota %s", resourceQuota.Name)
					return false, nil
				}
				log.Info("Created vm-operator ResourceQuota", "ResourceQuota", klog.KObj(resourceQuota))
			}
			return true, nil
		})
		if retryError != nil {
			return retryError
		}
	}

	// Create Availability zones CR in K8s and bind them to the user namespace
	// NOTE: For now we are creating one availability zone for the cluster as in the example cluster
	// TODO: investigate what options exists to create availability zones, and if we want to support more

	availabilityZone := &topologyv1.AvailabilityZone{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(config.Spec.VCenter.Cluster, "/")), "_", "-"), "/", "-"),
		},
		Spec: topologyv1.AvailabilityZoneSpec{
			ClusterComputeResourceMoId: cluster.Reference().Value,
			Namespaces: map[string]topologyv1.NamespaceInfo{
				config.Namespace: {
					PoolMoId:   resourcePool.Reference().Value,
					FolderMoId: folder.Reference().Value,
				},
			},
		},
	}

	_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		retryError = nil
		if err := c.Get(ctx, client.ObjectKeyFromObject(availabilityZone), availabilityZone); err != nil {
			if !apierrors.IsNotFound(err) {
				retryError = errors.Wrapf(err, "failed to get AvailabilityZone %s", availabilityZone.Name)
				return false, nil
			}
			if err := c.Create(ctx, availabilityZone); err != nil {
				retryError = errors.Wrapf(err, "failed to create AvailabilityZone %s", availabilityZone.Name)
				return false, nil
			}
			log.Info("Created vm-operator AvailabilityZone", "AvailabilityZone", klog.KObj(availabilityZone))
		}
		return true, nil
	})
	if retryError != nil {
		return retryError
	}

	if _, ok := availabilityZone.Spec.Namespaces[config.Namespace]; !ok {
		availabilityZone.Spec.Namespaces[config.Namespace] = topologyv1.NamespaceInfo{
			PoolMoId:   resourcePool.Reference().Value,
			FolderMoId: folder.Reference().Value,
		}
		if err := c.Update(ctx, availabilityZone); err != nil {
			return errors.Wrapf(err, "failed to update AvailabilityZone %s", availabilityZone.Name)
		}
		log.Info("Update vm-operator AvailabilityZone", "AvailabilityZone", klog.KObj(availabilityZone))
	}

	// Create vm-operator Secret in K8s
	// This secret contains credentials to access vCenter the vm-operator acts on.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmOperatorSecretName,
			Namespace: config.Spec.OperatorRef.Namespace,
		},
		Data: map[string][]byte{
			usernameSecretKey: []byte(config.Spec.VCenter.Username),
			passwordSecretKey: []byte(config.Spec.VCenter.Password),

			// Additional key we are adding to the VcCredsSecret for sake of convenience (not supported in vm-operator)
			thumbprintSecretKey: []byte(config.Spec.VCenter.Thumbprint),
		},
		Type: corev1.SecretTypeOpaque,
	}
	_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		retryError = nil
		if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			if !apierrors.IsNotFound(err) {
				retryError = errors.Wrapf(err, "failed to get vm-operator Secret %s", secret.Name)
				return false, nil
			}
			if err := c.Create(ctx, secret); err != nil {
				retryError = errors.Wrapf(err, "failed to create vm-operator Secret %s", secret.Name)
				return false, nil
			}
			log.Info("Created vm-operator Secret", "Secret", klog.KObj(secret))
		}
		return true, nil
	})
	if retryError != nil {
		return retryError
	}

	// Create vm-operator ConfigMap in K8s
	// This ConfigMap contains settings for the vm-operator instance.

	host, port, err := net.SplitHostPort(config.Spec.VCenter.ServerURL)
	if err != nil {
		return errors.Wrapf(err, "failed to split host %s", config.Spec.VCenter.ServerURL)
	}

	providerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: config.Spec.OperatorRef.Namespace,
		},
		Data: map[string]string{
			// caFilePathConfigMapKey:            "", // Leaving this empty because we don't have (yet) a solution to inject a CA file into the vm-operator pod.
			// datastoreConfigMapKey:             "", // It seems it is ok to leave it empty.
			datacenterConfigMapKey:            datacenter.Reference().Value,
			folderConfigMapKey:                folder.Reference().Value,
			insecureSkipTLSVerifyConfigMapKey: "true", // Using this given that we don't have (yet) a solution to inject a CA file into the vm-operator pod.
			// NetworkNameConfigMapKey:           config.Spec.VCenter.NetworkName, // It seems it is ok to leave it empty.
			resourcePoolConfigMapKey:         resourcePool.Reference().Value,
			storageClassRequiredConfigMapKey: "true",
			useInventoryConfigMapKey:         "false",
			credentialSecretNameConfigMapKey: secret.Name,
			hostConfigMapKey:                 host,
			portConfigMapKey:                 port,

			// Additional key we are adding to the vm-operator config map for sake of convenience (not supported in vm-operator)
			serverURLConfigMapKey:            config.Spec.VCenter.ServerURL,
			datacenterNameConfigMapKey:       config.Spec.VCenter.Datacenter,
			distributedPortGroupConfigMapKey: config.Spec.VCenter.DistributedPortGroupName,
		},
	}
	_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		retryError = nil
		if err := c.Get(ctx, client.ObjectKeyFromObject(providerConfigMap), providerConfigMap); err != nil {
			if !apierrors.IsNotFound(err) {
				retryError = errors.Wrapf(err, "failed to get vm-operator ConfigMap %s", providerConfigMap.Name)
				return false, nil
			}
			if err := c.Create(ctx, providerConfigMap); err != nil {
				retryError = errors.Wrapf(err, "failed to create vm-operator ConfigMap %s", providerConfigMap.Name)
				return false, nil
			}
			log.Info("Created vm-operator ConfigMap", "ConfigMap", klog.KObj(providerConfigMap))
		}
		return true, nil
	})
	if retryError != nil {
		return retryError
	}

	// Create VirtualMachineClass in K8s and bind it to the user namespace
	for _, vmc := range config.Spec.VirtualMachineClasses {
		vmClass := &vmoprv1.VirtualMachineClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmc.Name,
				Namespace: config.Namespace,
			},
			Spec: vmoprv1.VirtualMachineClassSpec{
				Hardware: vmoprv1.VirtualMachineClassHardware{
					Cpus:   vmc.Cpus,
					Memory: vmc.Memory,
				},
			},
		}
		_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			retryError = nil
			if err := c.Get(ctx, client.ObjectKeyFromObject(vmClass), vmClass); err != nil {
				if !apierrors.IsNotFound(err) {
					retryError = errors.Wrapf(err, "failed to get vm-operator VirtualMachineClass %s", vmClass.Name)
					return false, nil
				}
				if err := c.Create(ctx, vmClass); err != nil {
					retryError = errors.Wrapf(err, "failed to create vm-operator VirtualMachineClass %s", vmClass.Name)
					return false, nil
				}
				log.Info("Created vm-operator VirtualMachineClass", "VirtualMachineClass", klog.KObj(vmClass))
			}
			return true, nil
		})
		if retryError != nil {
			return retryError
		}

		vmClassBinding := &vmoprv1.VirtualMachineClassBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vmClass.Name,
				Namespace: config.Namespace,
			},
			ClassRef: vmoprv1.ClassReference{
				APIVersion: vmoprv1.SchemeGroupVersion.String(),
				Kind:       "VirtualMachineClass",
				Name:       vmClass.Name,
			},
		}
		_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			retryError = nil
			if err := c.Get(ctx, client.ObjectKeyFromObject(vmClassBinding), vmClassBinding); err != nil {
				if !apierrors.IsNotFound(err) {
					retryError = errors.Wrapf(err, "failed to get vm-operator VirtualMachineClassBinding %s", vmClassBinding.Name)
					return false, nil
				}
				if err := c.Create(ctx, vmClassBinding); err != nil {
					retryError = errors.Wrapf(err, "failed to create vm-operator VirtualMachineClassBinding %s", vmClassBinding.Name)
					return false, nil
				}
				log.Info("Created vm-operator VirtualMachineClassBinding", "VirtualMachineClassBinding", klog.KObj(vmClassBinding))
			}
			return true, nil
		})
		if retryError != nil {
			return retryError
		}
	}

	// Create a ContentLibrary in K8s and in vCenter, bind it to the K8s namespace
	// This requires a set of objects in vCenter(or vcsim) as well as their mapping in K8s
	// - vCenter: a Library containing an Item
	// - k8s: ContentLibraryProvider, ContentSource (both representing the library), a VirtualMachineImage (representing the Item)

	restClient := rest.NewClient(s.Client.Client)
	if err := restClient.Login(ctx, url.UserPassword(config.Spec.VCenter.Username, config.Spec.VCenter.Password)); err != nil {
		return errors.Wrap(err, "failed to login using the rest client")
	}

	libMgr := library.NewManager(restClient)

	contentLibrary := library.Library{
		Name: config.Spec.VCenter.ContentLibrary.Name,
		Type: "LOCAL",
		Storage: []library.StorageBackings{
			{
				DatastoreID: contentLibraryDatastore.Reference().Value,
				Type:        "DATASTORE",
			},
		},
	}
	libraries, err := libMgr.GetLibraries(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get ContentLibraries")
	}

	var contentLibraryID string
	if len(libraries) > 0 {
		for i := range libraries {
			if libraries[i].Name == contentLibrary.Name {
				contentLibraryID = libraries[i].ID
				break
			}
		}
	}

	if contentLibraryID == "" {
		id, err := libMgr.CreateLibrary(ctx, contentLibrary)
		if err != nil {
			return errors.Wrapf(err, "failed to create vm-operator ContentLibrary %s", contentLibrary.Name)
		}
		log.Info("Created vm-operator ContentLibrary in vCenter", "ContentLibrary", contentLibrary.Name)
		contentLibraryID = id
	}

	contentSource := &vmoprv1.ContentSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: contentLibraryID,
		},
		Spec: vmoprv1.ContentSourceSpec{
			ProviderRef: vmoprv1.ContentProviderReference{
				Name: contentLibraryID, // NOTE: this should match the ContentLibraryProvider name below
				Kind: "ContentLibraryProvider",
			},
		},
	}
	_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		retryError = nil
		if err := c.Get(ctx, client.ObjectKeyFromObject(contentSource), contentSource); err != nil {
			if !apierrors.IsNotFound(err) {
				retryError = errors.Wrapf(err, "failed to get vm-operator ContentSource %s", contentSource.Name)
				return false, nil
			}
			if err := c.Create(ctx, contentSource); err != nil {
				retryError = errors.Wrapf(err, "failed to create vm-operator ContentSource %s", contentSource.Name)
				return false, nil
			}
			log.Info("Created vm-operator ContentSource", "ContentSource", klog.KObj(contentSource))
		}
		return true, nil
	})
	if retryError != nil {
		return retryError
	}

	contentSourceBinding := &vmoprv1.ContentSourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      contentLibraryID,
			Namespace: config.Namespace,
		},
		ContentSourceRef: vmoprv1.ContentSourceReference{
			APIVersion: vmoprv1.SchemeGroupVersion.String(),
			Kind:       "ContentSource",
			Name:       contentSource.Name,
		},
	}
	_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		retryError = nil
		if err := c.Get(ctx, client.ObjectKeyFromObject(contentSourceBinding), contentSourceBinding); err != nil {
			if !apierrors.IsNotFound(err) {
				retryError = errors.Wrapf(err, "failed to get vm-operator ContentSourceBinding %s", contentSourceBinding.Name)
				return false, nil
			}
			if err := c.Create(ctx, contentSourceBinding); err != nil {
				retryError = errors.Wrapf(err, "failed to create vm-operator ContentSourceBinding %s", contentSourceBinding.Name)
				return false, nil
			}
			log.Info("Created vm-operator ContentSourceBinding", "ContentSourceBinding", klog.KObj(contentSourceBinding))
		}
		return true, nil
	})
	if retryError != nil {
		return retryError
	}

	contentLibraryProvider := &vmoprv1.ContentLibraryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: contentLibraryID,
		},
		Spec: vmoprv1.ContentLibraryProviderSpec{
			UUID: contentLibraryID,
		},
	}

	if err := controllerutil.SetOwnerReference(contentSource, contentLibraryProvider, c.Scheme()); err != nil {
		return errors.Wrap(err, "failed to set ContentLibraryProvider owner")
	}
	_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		retryError = nil
		if err := c.Get(ctx, client.ObjectKeyFromObject(contentSource), contentLibraryProvider); err != nil {
			if !apierrors.IsNotFound(err) {
				retryError = errors.Wrapf(err, "failed to get vm-operator ContentLibraryProvider %s", contentLibraryProvider.Name)
				return false, nil
			}
			if err := c.Create(ctx, contentLibraryProvider); err != nil {
				retryError = errors.Wrapf(err, "failed to create vm-operator ContentLibraryProvider %s", contentLibraryProvider.Name)
				return false, nil
			}
			log.Info("Created vm-operator ContentLibraryProvider", "ContentSource", klog.KObj(contentSource), "ContentLibraryProvider", klog.KObj(contentLibraryProvider))
		}
		return true, nil
	})
	if retryError != nil {
		return retryError
	}

	for _, item := range config.Spec.VCenter.ContentLibrary.Items {
		libraryItem := library.Item{
			Name:      item.Name,
			Type:      item.ItemType,
			LibraryID: contentLibraryID,
		}

		items, err := libMgr.GetLibraryItems(ctx, contentLibraryID)
		if err != nil {
			return errors.Wrap(err, "failed to get ContentLibraryItems")
		}

		var libraryItemID string
		for _, i := range items {
			if i.Name == libraryItem.Name {
				libraryItemID = i.ID
				break
			}
		}

		if libraryItemID == "" {
			id, err := libMgr.CreateLibraryItem(ctx, libraryItem)
			if err != nil {
				return errors.Wrapf(err, "failed to create vm-operator ContentLibraryItem %s", libraryItem.Name)
			}
			log.Info("Created vm-operator LibraryItem in vCenter", "ContentLibrary", contentLibrary.Name, "LibraryItem", libraryItem.Name)
			libraryItemID = id
		}

		virtualMachineImage := &vmoprv1.VirtualMachineImage{
			ObjectMeta: metav1.ObjectMeta{
				Name:      libraryItem.Name,
				Namespace: config.Namespace,
			},
			Spec: vmoprv1.VirtualMachineImageSpec{
				ImageID:         libraryItemID,
				ImageSourceType: "Content Library",
				Type:            "ovf",
				ProviderRef: vmoprv1.ContentProviderReference{
					APIVersion: vmoprv1.SchemeGroupVersion.String(),
					Kind:       "ContentLibraryItem",
					// Not 100% sure about following values now that Kind is required to be ContentLibraryItem, but this doesn't seem to be an issue
					Name:      contentLibraryProvider.Name,
					Namespace: contentLibraryProvider.Namespace,
				},
				ProductInfo: vmoprv1.VirtualMachineImageProductInfo{
					FullVersion: item.ProductInfo,
				},
				OSInfo: vmoprv1.VirtualMachineImageOSInfo{
					Type: item.OSInfo,
				},
			},
		}

		if err := controllerutil.SetOwnerReference(contentLibraryProvider, virtualMachineImage, c.Scheme()); err != nil {
			return errors.Wrap(err, "failed to set VirtualMachineImage owner")
		}
		_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			retryError = nil
			if err := c.Get(ctx, client.ObjectKeyFromObject(virtualMachineImage), virtualMachineImage); err != nil {
				if !apierrors.IsNotFound(err) {
					retryError = errors.Wrapf(err, "failed to get vm-operator VirtualMachineImage %s", virtualMachineImage.Name)
					return false, nil
				}
				if err := c.Create(ctx, virtualMachineImage); err != nil {
					retryError = errors.Wrapf(err, "failed to create vm-operator VirtualMachineImage %s", virtualMachineImage.Name)
					return false, nil
				}
				log.Info("Created vm-operator VirtualMachineImage", "ContentSource", klog.KObj(contentSource), "ContentLibraryProvider", klog.KObj(contentLibraryProvider), "VirtualMachineImage", klog.KObj(virtualMachineImage))
			}
			return true, nil
		})
		if retryError != nil {
			return retryError
		}

		// Fakes reconciliation of virtualMachineImage by setting required status field for the image to be considered ready.
		virtualMachineImageReconciled := virtualMachineImage.DeepCopy()
		virtualMachineImageReconciled.Status.ImageName = virtualMachineImage.Name
		Set(virtualMachineImageReconciled, TrueCondition(ReadyConditionType))
		_ = wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			retryError = nil
			if err := c.Status().Patch(ctx, virtualMachineImageReconciled, client.MergeFrom(virtualMachineImage)); err != nil {
				retryError = errors.Wrapf(err, "failed to patch vm-operator VirtualMachineImage %s", virtualMachineImage.Name)
			}
			log.Info("Patched vm-operator VirtualMachineImage", "ContentSource", klog.KObj(contentSource), "ContentLibraryProvider", klog.KObj(contentLibraryProvider), "VirtualMachineImage", klog.KObj(virtualMachineImage))
			return true, nil
		})
		if retryError != nil {
			return retryError
		}

		existingFiles, err := libMgr.ListLibraryItemFiles(ctx, libraryItemID)
		if err != nil {
			return errors.Wrapf(err, "failed to list files for vm-operator libraryItem %s", libraryItem.Name)
		}

		uploadFunc := func(sessionID, file string, content []byte) error {
			info := library.UpdateFile{
				Name:       file,
				SourceType: "PUSH",
				Size:       int64(len(content)),
			}

			update, err := libMgr.AddLibraryItemFile(ctx, sessionID, info)
			if err != nil {
				return err
			}

			u, err := url.Parse(update.UploadEndpoint.URI)
			if err != nil {
				return err
			}

			p := soap.DefaultUpload
			p.ContentLength = info.Size

			return libMgr.Client.Upload(ctx, bytes.NewReader(content), u, &p)
		}

		for _, file := range item.Files {
			exists := false
			for _, existingFile := range existingFiles {
				if file.Name == existingFile.Name {
					exists = true
				}
			}
			if exists {
				continue
			}

			sessionID, err := libMgr.CreateLibraryItemUpdateSession(ctx, library.Session{LibraryItemID: libraryItemID})
			if err != nil {
				return errors.Wrapf(err, "failed to start update session for vm-operator libraryItem %s", libraryItem.Name)
			}
			if err := uploadFunc(sessionID, file.Name, file.Content); err != nil {
				return errors.Wrapf(err, "failed to upload data for vm-operator libraryItem %s", libraryItem.Name)
			}
			if err := libMgr.CompleteLibraryItemUpdateSession(ctx, sessionID); err != nil {
				return errors.Wrapf(err, "failed to complete update session for vm-operator libraryItem %s", libraryItem.Name)
			}
			log.Info("Uploaded vm-operator LibraryItemFile in vCenter", "ContentLibrary", contentLibrary.Name, "libraryItem", libraryItem.Name, "LibraryItemFile", file.Name)
		}
	}

	return nil
}

// GetVCenterSession returns a VCenter session from vm-operator config.
func GetVCenterSession(ctx context.Context, c client.Client, enableKeepAlive bool, keepAliveDuration time.Duration) (*session.Session, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: DefaultNamespace, // This is where tilt/E2E deploy the vm-operator
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
		return nil, errors.Wrapf(err, "failed to get vm-operator ConfigMap %s", configMap.Name)
	}

	serverURL := configMap.Data[serverURLConfigMapKey]
	if serverURL == "" {
		return nil, errors.Errorf("%s value is missing from the vm-operator ConfigMap %s", serverURLConfigMapKey, configMap.Name)
	}
	datacenter := configMap.Data[datacenterNameConfigMapKey]
	if datacenter == "" {
		return nil, errors.Errorf("%s value is missing from the vm-operator ConfigMap %s", datacenterNameConfigMapKey, configMap.Name)
	}
	secretName := configMap.Data[credentialSecretNameConfigMapKey]
	if secretName == "" {
		return nil, errors.Errorf("%s value is missing from the vm-operator ConfigMap %s", credentialSecretNameConfigMapKey, configMap.Name)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: configMap.Namespace, // This is where tilt deploys the vm-operator
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return nil, errors.Wrapf(err, "failed to get vm-operator Credential Secret %s", secret.Name)
	}
	username := string(secret.Data[usernameSecretKey])
	if username == "" {
		return nil, errors.Errorf("%s value is missing from the vm-operator Secret %s", usernameSecretKey, secret.Name)
	}
	password := string(secret.Data[passwordSecretKey])
	if password == "" {
		return nil, errors.Errorf("%s value is missing from the vm-operator Secret %s", passwordSecretKey, secret.Name)
	}
	thumbprint := string(secret.Data[thumbprintSecretKey])
	if thumbprint == "" {
		return nil, errors.Errorf("%s value is missing from the vm-operator Secret %s", thumbprintSecretKey, secret.Name)
	}

	params := session.NewParams().
		WithServer(serverURL).
		WithDatacenter(datacenter).
		WithUserInfo(username, password).
		WithThumbprint(thumbprint).
		WithFeatures(session.Feature{
			EnableKeepAlive:   enableKeepAlive,
			KeepAliveDuration: keepAliveDuration,
		})

	return session.GetOrCreate(ctx, params)
}

// GetDistributedPortGroup returns a the DistributedPortGroup from vm-operator config.
func GetDistributedPortGroup(ctx context.Context, c client.Client) (string, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: DefaultNamespace, // This is where tilt/E2E deploy the vm-operator
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
		return "", errors.Wrapf(err, "failed to get vm-operator ConfigMap %s", configMap.Name)
	}

	distributedPortGroup := configMap.Data[distributedPortGroupConfigMapKey]
	if distributedPortGroup == "" {
		return "", errors.Errorf("%s value is missing from the vm-operator ConfigMap %s", distributedPortGroupConfigMapKey, configMap.Name)
	}

	return distributedPortGroup, nil
}

// NOTE: code below is a fork of vm-operator's pkg/conditions (so we can avoid to import the entire project).

const (
	ReadyConditionType = "Ready"
)

type Getter interface {
	client.Object

	// GetConditions returns the list of conditions for a cluster API object.
	GetConditions() vmoprv1.Conditions
}

type Setter interface {
	Getter
	SetConditions(vmoprv1.Conditions)
}

func Set(to Setter, condition *vmoprv1.Condition) {
	if to == nil || condition == nil {
		return
	}

	// Check if the new conditions already exists, and change it only if there is a status
	// transition (otherwise we should preserve the current last transition time)-
	conditions := to.GetConditions()
	exists := false
	for i := range conditions {
		existingCondition := conditions[i]
		if existingCondition.Type == condition.Type {
			exists = true
			if !hasSameState(&existingCondition, condition) {
				condition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
				conditions[i] = *condition
				break
			}
			condition.LastTransitionTime = existingCondition.LastTransitionTime
			break
		}
	}

	// If the condition does not exist, add it, setting the transition time only if not already set
	if !exists {
		if condition.LastTransitionTime.IsZero() {
			condition.LastTransitionTime = metav1.NewTime(time.Now().UTC().Truncate(time.Second))
		}
		conditions = append(conditions, *condition)
	}

	// Sorts conditions for convenience of the consumer, i.e. kubectl.
	sort.Slice(conditions, func(i, j int) bool {
		return lexicographicLess(&conditions[i], &conditions[j])
	})

	to.SetConditions(conditions)
}

func lexicographicLess(i, j *vmoprv1.Condition) bool {
	return (i.Type == ReadyConditionType || i.Type < j.Type) && j.Type != ReadyConditionType
}

func hasSameState(i, j *vmoprv1.Condition) bool {
	return i.Type == j.Type &&
		i.Status == j.Status &&
		i.Reason == j.Reason &&
		i.Message == j.Message
}

func TrueCondition(t vmoprv1.ConditionType) *vmoprv1.Condition {
	return &vmoprv1.Condition{
		Type:   t,
		Status: corev1.ConditionTrue,
		// This is a non-empty field in metav1.Conditions, when it was not in our v1a1 Conditions. This
		// really doesn't work with how we've defined our conditions so do something to make things
		// work for now.
		Reason: string(corev1.ConditionTrue),
	}
}
