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
	Provider_Datacenter              = "datacenter"
	Provider_Datastore               = "datastore"
	Provider_ResPool                 = "resource_pool"
	Provider_Network                 = "network"
	Provider_Template                = "vm_template"
	DefaultAPITimeout                = 5 * time.Minute
	VirtualMachineTaskRef            = "current-task-ref"
	VirtualMachineRef                = "vm-moid"
	KubeadmToken                     = "k8s-token"
	KubeadmTokenExpiryTime           = "k8s-token-expiry-time"
	KubeadmTokenTtl                  = 10 * time.Minute
	KubeadmTokenLeftTime             = 5 * time.Minute
	RequeueAfterSeconds              = 20 * time.Second
	KubeConfigSecretName             = "%s-kubeconfig"
	KubeConfigSecretData             = "admin-kubeconfig"
)
