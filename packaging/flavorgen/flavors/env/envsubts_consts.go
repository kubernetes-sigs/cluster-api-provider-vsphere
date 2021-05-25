package env

const (
	ClusterNameVar               = "${CLUSTER_NAME}"
	ControlPlaneMachineCountVar  = "${CONTROL_PLANE_MACHINE_COUNT}"
	DefaultCloudProviderImage    = "gcr.io/cloud-provider-vsphere/cpi/release/manager:v1.2.1"
	DefaultClusterCIDR           = "192.168.0.0/16"
	DefaultDiskGiB               = 25
	DefaultMemoryMiB             = 8192
	DefaultNumCPUs               = 2
	KubernetesVersionVar         = "${KUBERNETES_VERSION}"
	MachineDeploymentNameSuffix  = "-md-0"
	NamespaceVar                 = "${NAMESPACE}"
	VSphereDataCenterVar         = "${VSPHERE_DATACENTER}"
	VSphereThumbprint            = "${VSPHERE_TLS_THUMBPRINT}"
	VSphereDatastoreVar          = "${VSPHERE_DATASTORE}"
	VSphereFolderVar             = "${VSPHERE_FOLDER}"
	VSphereHaproxyTemplateVar    = "${VSPHERE_HAPROXY_TEMPLATE}"
	VSphereNetworkVar            = "${VSPHERE_NETWORK}"
	VSphereResourcePoolVar       = "${VSPHERE_RESOURCE_POOL}"
	VSphereServerVar             = "${VSPHERE_SERVER}"
	VSphereSSHAuthorizedKeysVar  = "${VSPHERE_SSH_AUTHORIZED_KEY}"
	VSphereStoragePolicyVar      = "${VSPHERE_STORAGE_POLICY}"
	VSphereTemplateVar           = "${VSPHERE_TEMPLATE}"
	WorkerMachineCountVar        = "${WORKER_MACHINE_COUNT}"
	ControlPlaneEndpointVar      = "${CONTROL_PLANE_ENDPOINT_IP}"
	VSphereUsername              = "${VSPHERE_USERNAME}"
	VSpherePassword              = "${VSPHERE_PASSWORD}" /* #nosec */
	ClusterResourceSetNameSuffix = "-crs-0"
)
