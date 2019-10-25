package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/NetApp/nks-on-prem-ipam/pkg/ipam"
	"github.com/NetApp/nks-on-prem-ipam/pkg/ipam/factory"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	capvcontext "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	primaryNetworkNameAnnotationKey = "primary-network-name"
	storageNetworkNameAnnotationKey = "storage-network-name"

	ipamConfigSecretNameAnnotationKey      = "ipam-config-secret-name"
	ipamConfigSecretNamespaceAnnotationKey = "ipam-config-secret-namespace"
	ipamConfigSecretKey                    = "config.json"

	ipamManagedAnnotationKey = "ipam-managed"

	managementZoneName = "management"
)

type IPAMService struct{}

func (svc *IPAMService) ReconcileIPAM(ctx *capvcontext.MachineContext) error {

	if ctx.VSphereMachine == nil {
		return fmt.Errorf("machine infrastructure missing")
	}

	// Create a copy of the devices
	devices := make([]infrav1.NetworkDeviceSpec, len(ctx.VSphereMachine.Spec.Network.Devices))
	for i := range ctx.VSphereMachine.Spec.Network.Devices {
		ctx.VSphereMachine.Spec.Network.Devices[i].DeepCopyInto(&devices[i])
	}

	// Find all devices that should get a static IP
	staticDevicesWithoutIPs := make([]*infrav1.NetworkDeviceSpec, 0)
	for i, device := range devices {
		if !device.DHCP4 && !device.DHCP6 && len(device.IPAddrs) == 0 {
			staticDevicesWithoutIPs = append(staticDevicesWithoutIPs, &devices[i])
		}
	}

	// If no static devices need IPs then nothing to do
	if len(staticDevicesWithoutIPs) == 0 {
		return nil
	}

	networkTypeDeviceMap, err := getNetworkTypeDeviceMap(ctx, staticDevicesWithoutIPs)
	if err != nil {
		return errors.Wrap(err, "could not get network types for devices")
	}

	agent, err := getIPAMAgent(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get ipam agent")
	}

	for networkType, networkTypeDevices := range networkTypeDeviceMap {

		reservations, err := agent.ReserveIPs(networkType, ipam.IPv4, len(networkTypeDevices), nil)
		if err != nil {
			return errors.Wrapf(err, "could not reserve IPs for network type %q", string(networkType))
		}
		ctx.Logger.Info("Reserved IPs", "networkType", string(networkType), "IPs", getReservationIPs(reservations))

		// Update IPAM annotation

		networkTypeToIPs := make(map[string][]string)
		val, ok := ctx.VSphereMachine.Annotations[ipamManagedAnnotationKey]
		if ok {
			if err := json.Unmarshal([]byte(val), &networkTypeToIPs); err != nil {
				cleanupReservations(ctx, agent, networkType, reservations)
				return errors.Wrap(err, "failed to unmarshal ipam state annotation")
			}
		}
		networkTypeIPs, ok := networkTypeToIPs[string(networkType)]
		if ok {
			networkTypeToIPs[string(networkType)] = append(networkTypeIPs, getReservationIPs(reservations)...)
		} else {
			networkTypeToIPs[string(networkType)] = getReservationIPs(reservations)
		}
		marshalled, err := json.Marshal(networkTypeToIPs)
		if err != nil {
			cleanupReservations(ctx, agent, networkType, reservations)
			return errors.Wrap(err, "failed to marshal ipam state annotation")
		}
		ctx.VSphereMachine.Annotations[ipamManagedAnnotationKey] = string(marshalled)

		// Assign to devices

		if err := assignReservationsToDevices(reservations, networkTypeDevices); err != nil {
			cleanupReservations(ctx, agent, networkType, reservations)
			ctx.VSphereMachine.Annotations[ipamManagedAnnotationKey] = val // Revert annotation
			return errors.Wrap(err, "could not assign IP reservations to devices")
		}
		// Assign the modified devices
		ctx.VSphereMachine.Spec.Network.Devices = devices
	}

	return nil
}

// ReleaseIPAM releases the IP addresses specified in the ipam-managed annotation on the VSphereMachine
func (svc *IPAMService) ReleaseIPAM(ctx *capvcontext.MachineContext) error {

	if ctx.VSphereMachine == nil {
		return fmt.Errorf("machine infrastructure missing")
	}

	// If this machine is not IPAM managed we can bail early
	val, ok := ctx.VSphereMachine.Annotations[ipamManagedAnnotationKey]
	if !ok {
		return nil
	}

	networkTypeToIPs := make(map[string][]string)
	if err := json.Unmarshal([]byte(val), &networkTypeToIPs); err != nil {
		return errors.Wrap(err, "failed to unmarshal ipam state annotation")
	}

	if len(networkTypeToIPs) == 0 {
		// Nothing to do
		return nil
	}

	agent, err := getIPAMAgent(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get ipam agent")
	}

	for netType, ips := range networkTypeToIPs {
		networkType, err := mapNetworkType(netType)
		if err != nil {
			return errors.Wrapf(err, "could not map network type")
		}
		if len(ips) > 0 {
			if err := agent.ReleaseIPs(networkType, ips); err != nil {
				return errors.Wrapf(err, "could not release IPs: %s", ips)
			}
			ctx.Logger.Info("Released IPs", "networkType", string(networkType), "IPs", ips)
		}
		delete(networkTypeToIPs, netType)
		marshalled, err := json.Marshal(networkTypeToIPs)
		if err != nil {
			return errors.Wrap(err, "failed to marshal ipam state annotation")
		}
		ctx.VSphereMachine.Annotations[ipamManagedAnnotationKey] = string(marshalled)
	}

	return nil
}

func cleanupReservations(ctx *capvcontext.MachineContext, agent ipam.Agent, networkType ipam.NetworkType, reservations []ipam.IPAddressReservation) {
	ips := getReservationIPs(reservations)
	ctx.Logger.Info("Cleaning up IP reservations", "networkType", string(networkType), "IPs", ips)
	if err := agent.ReleaseIPs(networkType, ips); err != nil {
		ctx.Logger.Error(err, "failed to clean up reservations, could not release IPs", "IPs", ips)
	}
}

func getNetworkType(machine infrav1.VSphereMachine, managementZone bool, networkName string) (ipam.NetworkType, error) {
	primaryNetworkName, ok := machine.Annotations[primaryNetworkNameAnnotationKey]
	if !ok {
		return ipam.Workload, fmt.Errorf("primary network name annotation missing")

	}
	storageNetworkName, ok := machine.Annotations[storageNetworkNameAnnotationKey]
	if !ok {
		return ipam.Workload, fmt.Errorf("storage network name annotation missing")

	}
	if networkName == primaryNetworkName {
		if managementZone {
			return ipam.Management, nil
		}
		return ipam.Workload, nil
	}
	if networkName == storageNetworkName {
		return ipam.Data, nil
	}
	return ipam.Workload, fmt.Errorf("unknown network type for network %q", networkName)
}

func mapNetworkType(networkType string) (ipam.NetworkType, error) {
	if networkType == string(ipam.Management) {
		return ipam.Management, nil
	}
	if networkType == string(ipam.Workload) {
		return ipam.Workload, nil
	}
	if networkType == string(ipam.Data) {
		return ipam.Data, nil
	}
	return ipam.Workload, fmt.Errorf("unknown network type %q", networkType)
}

func getNetworkTypeDeviceMap(ctx *capvcontext.MachineContext, devices []*infrav1.NetworkDeviceSpec) (map[ipam.NetworkType][]*infrav1.NetworkDeviceSpec, error) {

	// Determine zone
	if ctx.VSphereCluster == nil {
		return nil, fmt.Errorf("cluster infrastructure missing")
	}

	isManagementZone := ctx.VSphereCluster.Spec.CloudProviderConfiguration.Labels.Zone == managementZoneName

	// Determine network types for each device
	networkTypeDeviceMap := make(map[ipam.NetworkType][]*infrav1.NetworkDeviceSpec)
	for i := range devices {
		networkType, err := getNetworkType(*ctx.VSphereMachine, isManagementZone, devices[i].NetworkName)
		if err != nil {
			return nil, errors.Wrapf(err, "could not get network type")
		}
		networkTypeDeviceMap[networkType] = append(networkTypeDeviceMap[networkType], devices[i])
	}

	return networkTypeDeviceMap, nil
}

func assignReservationsToDevices(reservations []ipam.IPAddressReservation, devices []*infrav1.NetworkDeviceSpec) error {
	if len(reservations) != len(devices) {
		return fmt.Errorf("unexpected number of reservations %d, wanted %d", len(reservations), len(devices))
	}
	for i, device := range devices {
		reservation := reservations[i]
		device.IPAddrs = append(device.IPAddrs, reservation.Address)
		device.Nameservers = reservation.NetworkConfig.NameServers
		device.Gateway4 = reservation.NetworkConfig.DefaultGateway
		device.SearchDomains = reservation.NetworkConfig.DomainSearch
	}
	return nil
}

func getReservationIPs(reservations []ipam.IPAddressReservation) []string {
	ips := make([]string, 0)
	for _, res := range reservations {
		ips = append(ips, res.Address)
	}
	return ips
}

func getIPAMAgent(ctx *capvcontext.MachineContext) (ipam.Agent, error) {
	cfg, err := loadConfig(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get ipam config")
	}
	agent, err := factory.GetAgent(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "could not get ipam agent")
	}
	if err := agent.HealthCheck(); err != nil {
		return nil, errors.Wrap(err, "ipam agent health check failed")
	}
	return agent, nil
}

func loadConfig(ctx *capvcontext.MachineContext) (*ipam.Config, error) {

	if ctx.Cluster == nil {
		return nil, fmt.Errorf("cluster context is nil")
	}

	secretName, ok := ctx.Cluster.Annotations[ipamConfigSecretNameAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("ipam config secret name annotation missing")
	}
	secretNamespace, ok := ctx.Cluster.Annotations[ipamConfigSecretNamespaceAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("ipam config secret namespace annotation missing")
	}

	ctx.Logger.V(4).Info("Fetching IPAM configuration from secret", "secret-namespace", secretNamespace, "secret-name", secretName)

	ipamSecret := &apiv1.Secret{}
	ipamSecretKey := client.ObjectKey{
		Namespace: secretNamespace,
		Name:      secretName,
	}
	if err := ctx.Client.Get(context.TODO(), ipamSecretKey, ipamSecret); err != nil {
		return nil, errors.Wrapf(err, "error getting ipam config secret %s in namespace %s", secretName, secretNamespace)
	}

	configBytes, ok := ipamSecret.Data[ipamConfigSecretKey]
	if !ok {
		return nil, fmt.Errorf("ipam config missing from secret %s in namespace %s", secretName, secretNamespace)
	}

	cfg := &ipam.Config{}
	err := json.Unmarshal(configBytes, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "could not unmarshal ipam config")
	}

	return cfg, nil
}
