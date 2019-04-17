package constants

import (
	"time"
)

const (
	ApiServerPort                    = 443
	VmIpAnnotationKey                = "vm-ip-address"
	ControlPlaneVersionAnnotationKey = "control-plane-version"
	KubeletVersionAnnotationKey      = "kubelet-version"
	CreateEventAction                = "Create"
	DeleteEventAction                = "Delete"
	DefaultAPITimeout                = 5 * time.Minute
	VirtualMachineTaskRef            = "current-task-ref"
	KubeadmToken                     = "k8s-token"
	KubeadmTokenExpiryTime           = "k8s-token-expiry-time"
	KubeadmTokenTtl                  = 10 * time.Minute
	KubeadmTokenLeftTime             = 5 * time.Minute
	RequeueAfterSeconds              = 20 * time.Second
	KubeConfigSecretName             = "%s-kubeconfig"
	KubeConfigSecretData             = "admin-kubeconfig"
	VsphereUserKey                   = "username"
	VspherePasswordKey               = "password"
	ClusterIsNullErr                 = "cluster is nil, make sure machines have `clusters.k8s.io/cluster-name` label set and the name references a valid cluster name in the same namespace"
)
