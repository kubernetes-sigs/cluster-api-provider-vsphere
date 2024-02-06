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
	"strings"

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
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

const DefaultNamespace = "vmware-system-vmop"

const (
	// NOTE: const below are copied from pkg/vmprovider/providers/vsphere/config/config.go.
	// int the vm-operator project.

	providerConfigMapName    = "vsphere.provider.config.vmoperator.vmware.com"
	vcPNIDKey                = "VcPNID"
	vcPortKey                = "VcPort"
	vcCredsSecretNameKey     = "VcCredsSecretName" //nolint:gosec
	datacenterKey            = "Datacenter"
	resourcePoolKey          = "ResourcePool"
	folderKey                = "Folder"
	datastoreKey             = "Datastore"
	networkNameKey           = "Network"
	scRequiredKey            = "StorageClassRequired"
	useInventoryKey          = "UseInventoryAsContentSource"
	insecureSkipTLSVerifyKey = "InsecureSkipTLSVerify"
	caFilePathKey            = "CAFilePath"
)

type ContentLibraryItemFilesConfig struct {
	Name    string
	Content []byte
}

type ContentLibraryItemConfig struct {
	Name        string
	Files       []ContentLibraryItemFilesConfig
	ItemType    string
	ProductInfo string
	OSInfo      string
}

type ContentLibraryConfig struct {
	Name      string
	Datastore string
	Item      ContentLibraryItemConfig
}

type VCenterClusterConfig struct {
	ServerURL  string
	Username   string
	Password   string
	Thumbprint string

	// supervisor is based on a single vCenter cluster
	Datacenter     string
	Cluster        string
	Folder         string
	ResourcePool   string
	StoragePolicy  string
	ContentLibrary ContentLibraryConfig
}

type UserNamespaceConfig struct {
	Name                string
	StorageClass        string
	VirtualMachineClass string
}

// Dependencies models dependencies for the vm-operator.
type Dependencies struct {
	// This is the namespace where is deployed the vm-operator
	Namespace string

	// Info about the vCenter cluster the vm-operator is bound to
	VCenterCluster VCenterClusterConfig

	// Info about where the users are expected to store Cluster API clusters to be managed by the vm-operator
	UserNamespace UserNamespaceConfig
}

// ReconcileDependencies reconciles dependencies for the vm-operator.
// NOTE: This func is idempotent, it creates objects if missing otherwise it uses existing ones
// (this will allow e.g. to update images once and re-use for many test run).
func ReconcileDependencies(ctx context.Context, c client.Client, config *Dependencies) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling dependencies for the VMOperator Deployment")

	// Get a Client to VCenter and get holds on the relevant objects that should already exist
	params := session.NewParams().
		WithServer(config.VCenterCluster.ServerURL).
		WithThumbprint(config.VCenterCluster.Thumbprint).
		WithUserInfo(config.VCenterCluster.Username, config.VCenterCluster.Password)

	s, err := session.GetOrCreate(ctx, params)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to vCenter Server instance to read dependency references")
	}

	datacenter, err := s.Finder.Datacenter(ctx, config.VCenterCluster.Datacenter)
	if err != nil {
		return errors.Wrapf(err, "failed to get datacenter %s", config.VCenterCluster.Datacenter)
	}

	cluster, err := s.Finder.ClusterComputeResource(ctx, config.VCenterCluster.Cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to get cluster %s", config.VCenterCluster.Cluster)
	}

	folder, err := s.Finder.Folder(ctx, config.VCenterCluster.Folder)
	if err != nil {
		return errors.Wrapf(err, "failed to get folder %s", config.VCenterCluster.Folder)
	}

	resourcePool, err := s.Finder.ResourcePool(ctx, config.VCenterCluster.ResourcePool)
	if err != nil {
		return errors.Wrapf(err, "failed to get resourcePool %s", config.VCenterCluster.ResourcePool)
	}

	contentLibraryDatastore, err := s.Finder.Datastore(ctx, config.VCenterCluster.ContentLibrary.Datastore)
	if err != nil {
		return errors.Wrapf(err, "failed to get contentLibraryDatastore %s", config.VCenterCluster.ContentLibrary.Datastore)
	}

	pbmClient, err := pbm.NewClient(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "failed to get storage policy client")
	}

	storagePolicyID, err := pbmClient.ProfileIDByName(ctx, config.VCenterCluster.StoragePolicy)
	if err != nil {
		return errors.Wrapf(err, "failed to get storage policy profile %s", config.VCenterCluster.StoragePolicy)
	}

	// Create StorageClass & bind it to the user namespace via a ResourceQuota
	// NOTE: vm-operator is using the ResourceQuota to figure out which StorageClass can be used from a namespace
	// TODO: consider if we want to support more than one storage class

	storageClass := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: config.UserNamespace.StorageClass,
		},
		Provisioner: "kubernetes.io/vsphere-volume",
		Parameters: map[string]string{
			"storagePolicyID": storagePolicyID,
		},
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(storageClass), storageClass); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator StorageClass %s", storageClass.Name)
		}

		if err := c.Create(ctx, storageClass); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator StorageClass %s", storageClass.Name)
		}
		log.Info("Created vm-operator StorageClass", "StorageClass", klog.KObj(storageClass))
	}

	// TODO: rethink about this, for now we are creating a ResourceQuota with the same name of the StorageClass, might be this is not ok when hooking into a real vCenter
	resourceQuota := &corev1.ResourceQuota{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.UserNamespace.StorageClass,
			Namespace: config.UserNamespace.Name,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceName(fmt.Sprintf("%s.storageclass.storage.k8s.io/requests.storage", storageClass.Name)): resource.MustParse("1Gi"),
			},
		},
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(resourceQuota), resourceQuota); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator ResourceQuota %s", resourceQuota.Name)
		}

		if err := c.Create(ctx, resourceQuota); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator ResourceQuota %s", resourceQuota.Name)
		}
		log.Info("Created vm-operator ResourceQuota", "ResourceQuota", klog.KObj(resourceQuota))
	}

	// Create Availability zones CR in K8s and bind them to the user namespace
	// NOTE: For now we are creating one availability zone for the cluster as in the example cluster
	// TODO: investigate what options exists to create availability zones, and if we want to support more

	availabilityZone := &topologyv1.AvailabilityZone{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(config.VCenterCluster.Cluster, "/")), "_", "-"), "/", "-"),
		},
		Spec: topologyv1.AvailabilityZoneSpec{
			ClusterComputeResourceMoId: cluster.Reference().Value,
			Namespaces: map[string]topologyv1.NamespaceInfo{
				config.UserNamespace.Name: {
					PoolMoId:   resourcePool.Reference().Value,
					FolderMoId: folder.Reference().Value,
				},
			},
		},
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(availabilityZone), availabilityZone); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get AvailabilityZone %s", availabilityZone.Name)
		}

		if err := c.Create(ctx, availabilityZone); err != nil {
			return errors.Wrapf(err, "failed to create AvailabilityZone %s", availabilityZone.Name)
		}
		log.Info("Created vm-operator AvailabilityZone", "AvailabilityZone", klog.KObj(availabilityZone))
	}

	// Create vm-operator Secret in K8s
	// This secret contains credentials to access vCenter the vm-operator acts on.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerConfigMapName, // using the same name of the config map for consistency.
			Namespace: config.Namespace,
		},
		Data: map[string][]byte{
			"username": []byte(config.VCenterCluster.Username),
			"password": []byte(config.VCenterCluster.Password),
		},
		Type: corev1.SecretTypeOpaque,
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator Secret %s", secret.Name)
		}
		if err := c.Create(ctx, secret); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator Secret %s", secret.Name)
		}
		log.Info("Created vm-operator Secret", "Secret", klog.KObj(secret))
	}

	// Create vm-operator ConfigMap in K8s
	// This ConfigMap contains settings for the vm-operator instance.

	host, port, err := net.SplitHostPort(config.VCenterCluster.ServerURL)
	if err != nil {
		return errors.Wrapf(err, "failed to split host %s", config.VCenterCluster.ServerURL)
	}

	providerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerConfigMapName,
			Namespace: config.Namespace,
		},
		Data: map[string]string{
			caFilePathKey:            "", // Leaving this empty because we don't have (yet) a solution to inject a CA file into the vm-operator pod.
			datastoreKey:             "", // It seems it is ok to leave it empty.
			datacenterKey:            datacenter.Reference().Value,
			folderKey:                folder.Reference().Value,
			insecureSkipTLSVerifyKey: "true", // Using this given that we don't have (yet) a solution to inject a CA file into the vm-operator pod.
			networkNameKey:           "",     // It seems it is ok to leave it empty.
			resourcePoolKey:          resourcePool.Reference().Value,
			scRequiredKey:            "true",
			useInventoryKey:          "false",
			vcCredsSecretNameKey:     secret.Name,
			vcPNIDKey:                host,
			vcPortKey:                port,
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(providerConfigMap), providerConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator ConfigMap %s", providerConfigMap.Name)
		}
		if err := c.Create(ctx, providerConfigMap); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator ConfigMap %s", providerConfigMap.Name)
		}
		log.Info("Created vm-operator ConfigMap", "ConfigMap", klog.KObj(providerConfigMap))
	}

	// Create VirtualMachineClass in K8s and bind it to the user namespace
	// TODO: figure out if to add more vm classes / if to make them configurable via config
	vmClass := &vmoprv1.VirtualMachineClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.UserNamespace.VirtualMachineClass,
		},
		Spec: vmoprv1.VirtualMachineClassSpec{
			Hardware: vmoprv1.VirtualMachineClassHardware{
				Cpus:   8,
				Memory: resource.MustParse("64G"),
			},
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(vmClass), vmClass); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator VirtualMachineClass %s", vmClass.Name)
		}
		if err := c.Create(ctx, vmClass); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator VirtualMachineClass %s", vmClass.Name)
		}
		log.Info("Created vm-operator VirtualMachineClass", "VirtualMachineClass", klog.KObj(vmClass))
	}

	vmClassBinding := &vmoprv1.VirtualMachineClassBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmClass.Name,
			Namespace: config.UserNamespace.Name,
		},
		ClassRef: vmoprv1.ClassReference{
			APIVersion: vmoprv1.SchemeGroupVersion.String(),
			Kind:       "VirtualMachineClass",
			Name:       vmClass.Name,
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(vmClassBinding), vmClassBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator VirtualMachineClassBinding %s", vmClassBinding.Name)
		}
		if err := c.Create(ctx, vmClassBinding); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator VirtualMachineClassBinding %s", vmClassBinding.Name)
		}
		log.Info("Created vm-operator VirtualMachineClassBinding", "VirtualMachineClassBinding", klog.KObj(vmClassBinding))
	}

	// Create a ContentLibrary in K8s and in vCenter, bind it to the K8s namespace
	// This requires a set of objects in vCenter(or vcsim) as well as their mapping in K8s
	// - vCenter: a Library containing an Item
	// - k8s: ContentLibraryProvider, ContentSource (both representing the library), a VirtualMachineImage (representing the Item)

	restClient := rest.NewClient(s.Client.Client)
	if err := restClient.Login(ctx, url.UserPassword(config.VCenterCluster.Username, config.VCenterCluster.Password)); err != nil {
		return errors.Wrap(err, "failed to login using the rest client")
	}

	libMgr := library.NewManager(restClient)

	contentLibrary := library.Library{
		Name: config.VCenterCluster.ContentLibrary.Name,
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
	if err := c.Get(ctx, client.ObjectKeyFromObject(contentSource), contentSource); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator ContentSource %s", contentSource.Name)
		}
		if err := c.Create(ctx, contentSource); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator ContentSource %s", contentSource.Name)
		}
		log.Info("Created vm-operator ContentSource", "ContentSource", klog.KObj(contentSource))
	}

	contentSourceBinding := &vmoprv1.ContentSourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      contentLibraryID,
			Namespace: config.UserNamespace.Name,
		},
		ContentSourceRef: vmoprv1.ContentSourceReference{
			APIVersion: vmoprv1.SchemeGroupVersion.String(),
			Kind:       "ContentSource",
			Name:       contentSource.Name,
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(contentSourceBinding), contentSourceBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator ContentSourceBinding %s", contentSourceBinding.Name)
		}
		if err := c.Create(ctx, contentSourceBinding); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator ContentSourceBinding %s", contentSourceBinding.Name)
		}
		log.Info("Created vm-operator ContentSourceBinding", "ContentSourceBinding", klog.KObj(contentSourceBinding))
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
	if err := c.Get(ctx, client.ObjectKeyFromObject(contentSource), contentLibraryProvider); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator ContentLibraryProvider %s", contentLibraryProvider.Name)
		}
		if err := c.Create(ctx, contentLibraryProvider); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator ContentLibraryProvider %s", contentLibraryProvider.Name)
		}
		log.Info("Created vm-operator ContentLibraryProvider", "ContentSource", klog.KObj(contentSource), "ContentLibraryProvider", klog.KObj(contentLibraryProvider))
	}

	libraryItem := library.Item{
		Name:      config.VCenterCluster.ContentLibrary.Item.Name,
		Type:      config.VCenterCluster.ContentLibrary.Item.ItemType,
		LibraryID: contentLibraryID,
	}

	items, err := libMgr.GetLibraryItems(ctx, contentLibraryID)
	if err != nil {
		return errors.Wrap(err, "failed to get ContentLibraryItems")
	}

	var libraryItemID string
	for _, item := range items {
		if item.Name == libraryItem.Name {
			libraryItemID = item.ID
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
			Name: libraryItem.Name,
		},
		Spec: vmoprv1.VirtualMachineImageSpec{
			ProductInfo: vmoprv1.VirtualMachineImageProductInfo{
				FullVersion: config.VCenterCluster.ContentLibrary.Item.ProductInfo,
			},
			OSInfo: vmoprv1.VirtualMachineImageOSInfo{
				Type: config.VCenterCluster.ContentLibrary.Item.OSInfo,
			},
		},
	}

	if err := controllerutil.SetOwnerReference(contentLibraryProvider, virtualMachineImage, c.Scheme()); err != nil {
		return errors.Wrap(err, "failed to set VirtualMachineImage owner")
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(virtualMachineImage), virtualMachineImage); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get vm-operator VirtualMachineImage %s", virtualMachineImage.Name)
		}
		if err := c.Create(ctx, virtualMachineImage); err != nil {
			return errors.Wrapf(err, "failed to create vm-operator VirtualMachineImage %s", virtualMachineImage.Name)
		}
		log.Info("Created vm-operator VirtualMachineImage", "ContentSource", klog.KObj(contentSource), "ContentLibraryProvider", klog.KObj(contentLibraryProvider), "VirtualMachineImage", klog.KObj(virtualMachineImage))
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

	for _, file := range config.VCenterCluster.ContentLibrary.Item.Files {
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
	return nil
}
