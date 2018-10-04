package constants

import "time"

const (
	ApiServerPort                    = 443
	VmIpAnnotationKey                = "vm-ip-address"
	ControlPlaneVersionAnnotationKey = "control-plane-version"
	KubeletVersionAnnotationKey      = "kubelet-version"
	LastUpdatedKey                   = "last-updated"
	CreateEventAction                = "Create"
	DeleteEventAction                = "Delete"
	RequeueAfterSeconds              = 20 * time.Second
)
