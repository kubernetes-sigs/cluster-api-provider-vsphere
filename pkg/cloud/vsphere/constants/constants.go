package constants

import (
	"time"
)

const (
	ApiServerPort                    = 443
	VmIpAnnotationKey                = "vm-ip-address"
	ControlPlaneVersionAnnotationKey = "control-plane-version"
	KubeletVersionAnnotationKey      = "kubelet-version"
	LastUpdatedKey                   = "last-updated"
	CreateEventAction                = "Create"
	DeleteEventAction                = "Delete"
	ProviderDatacenter               = "datacenter"
	ProviderDatastore                = "datastore"
	ProviderVmFolder                 = "vm_folder"
	ProviderResPool                  = "resource_pool"
	ProviderNetwork                  = "network"
	ProviderTemplate                 = "vm_template"
	DefaultAPITimeout                = 5 * time.Minute
	VirtualMachineTaskRef            = "current-task-ref"
	VirtualMachineRef                = "vm-moid"
	KubeadmToken                     = "k8s-token"
	KubeadmTokenExpiryTime           = "k8s-token-expiry-time"
	KubeadmTokenTtl                  = 20 * time.Minute
	KubeadmTokenLeftTime             = 15 * time.Minute
	RequeueAfterSeconds              = 20 * time.Second
	KubeConfigSecretName             = "%s-kubeconfig"
	KubeConfigSecretData             = "admin-kubeconfig"
)
