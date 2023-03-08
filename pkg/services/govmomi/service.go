/*
Copyright 2019 The Kubernetes Authors.

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

package govmomi

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"strings"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/pbm"
	pbmTypes "github.com/vmware/govmomi/pbm/types"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/cluster"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/clustermodules"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/extra"
	govmominet "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/net"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/pci"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/util"
)

// VMService provdes API to interact with the VMs using govmomi.
type VMService struct{}

// ipamDeviceConfig aids and holds state for the process of parsing IPAM
// addresses for a given device.
type ipamDeviceConfig struct {
	DeviceIndex         int
	IPAMAddresses       []*ipamv1.IPAddress
	MACAddress          string
	NetworkSpecGateway4 string
	IPAMConfigGateway4  string
	NetworkSpecGateway6 string
	IPAMConfigGateway6  string
}

// ReconcileVM makes sure that the VM is in the desired state by:
//  1. Creating the VM if it does not exist, then...
//  2. Updating the VM with the bootstrap data, such as the cloud-init meta and user data, before...
//  3. Powering on the VM, and finally...
//  4. Returning the real-time state of the VM to the caller
func (vms *VMService) ReconcileVM(ctx *context.VMContext) (vm infrav1.VirtualMachine, _ error) {
	// Initialize the result.
	vm = infrav1.VirtualMachine{
		Name:  ctx.VSphereVM.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	// If there is an in-flight task associated with this VM then do not
	// reconcile the VM until the task is completed.
	if inFlight, err := reconcileInFlightTask(ctx); err != nil || inFlight {
		return vm, err
	}

	// This deferred function will trigger a reconcile event for the
	// VSphereVM resource once its associated task completes. If
	// there is no task for the VSphereVM resource then no reconcile
	// event is triggered.
	defer reconcileVSphereVMOnTaskCompletion(ctx)

	if ok, err := vms.reconcileIPAddressClaims(ctx); err != nil || !ok {
		return vm, err
	}

	// Before going further, we need the VM's managed object reference.
	vmRef, err := findVM(ctx)
	//nolint:nestif
	if err != nil {
		if !isNotFound(err) {
			return vm, err
		}

		// If the machine was not found by BIOS UUID it means that it got deleted from vcenter directly
		if wasNotFoundByBIOSUUID(err) {
			ctx.VSphereVM.Status.FailureReason = capierrors.MachineStatusErrorPtr(capierrors.UpdateMachineError)
			ctx.VSphereVM.Status.FailureMessage = pointer.StringPtr(fmt.Sprintf("Unable to find VM by BIOS UUID %s. The vm was removed from infra", ctx.VSphereVM.Spec.BiosUUID))
			return vm, err
		}

		// Otherwise, this is a new machine and the  the VM should be created.
		// NOTE: We are setting this condition only in case it does not exists so we avoid to get flickering LastConditionTime
		// in case of cloning errors or powering on errors.
		if !conditions.Has(ctx.VSphereVM, infrav1.VMProvisionedCondition) {
			conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, infrav1.CloningReason, clusterv1.ConditionSeverityInfo, "")
		}

		// Get the bootstrap data.
		bootstrapData, format, err := vms.getBootstrapData(ctx)
		if err != nil {
			conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, infrav1.CloningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return vm, err
		}

		// Create the VM.
		err = createVM(ctx, bootstrapData, format)
		if err != nil {
			conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, infrav1.CloningFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return vm, err
		}
		return vm, nil
	}

	//
	// At this point we know the VM exists, so it needs to be updated.
	//

	// Create a new virtualMachineContext to reconcile the VM.
	vmCtx := &virtualMachineContext{
		VMContext: *ctx,
		Obj:       object.NewVirtualMachine(ctx.Session.Client.Client, vmRef),
		Ref:       vmRef,
		State:     &vm,
	}

	vms.reconcileUUID(vmCtx)

	if ok, err := vms.reconcileHardwareVersion(vmCtx); err != nil || !ok {
		return vm, err
	}

	if err := vms.reconcilePCIDevices(vmCtx); err != nil {
		return vm, err
	}

	if err := vms.reconcileNetworkStatus(vmCtx); err != nil {
		return vm, err
	}

	if ok, err := vms.reconcileIPAddresses(vmCtx); err != nil || !ok {
		return vm, err
	}

	if ok, err := vms.reconcileMetadata(vmCtx); err != nil || !ok {
		return vm, err
	}

	if err := vms.reconcileStoragePolicy(vmCtx); err != nil {
		return vm, err
	}

	if ok, err := vms.reconcileVMGroupInfo(vmCtx); err != nil || !ok {
		return vm, err
	}

	if err := vms.reconcileClusterModuleMembership(vmCtx); err != nil {
		return vm, err
	}

	if ok, err := vms.reconcilePowerState(vmCtx); err != nil || !ok {
		return vm, err
	}

	if err := vms.reconcileHostInfo(vmCtx); err != nil {
		return vm, err
	}

	if err := vms.reconcileTags(vmCtx); err != nil {
		conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, infrav1.TagsAttachmentFailedReason, clusterv1.ConditionSeverityError, err.Error())
		return vm, err
	}

	vm.State = infrav1.VirtualMachineStateReady
	return vm, nil
}

// DestroyVM powers off and destroys a virtual machine.
func (vms *VMService) DestroyVM(ctx *context.VMContext) (infrav1.VirtualMachine, error) {
	vm := infrav1.VirtualMachine{
		Name:  ctx.VSphereVM.Name,
		State: infrav1.VirtualMachineStatePending,
	}

	// If there is an in-flight task associated with this VM then do not
	// reconcile the VM until the task is completed.
	if inFlight, err := reconcileInFlightTask(ctx); err != nil || inFlight {
		return vm, err
	}

	// This deferred function will trigger a reconcile event for the
	// VSphereVM resource once its associated task completes. If
	// there is no task for the VSphereVM resource then no reconcile
	// event is triggered.
	defer reconcileVSphereVMOnTaskCompletion(ctx)

	// Before going further, we need the VM's managed object reference.
	vmRef, err := findVM(ctx)
	if err != nil {
		// If the VM's MoRef could not be found then the VM no longer exists. This
		// is the desired state.
		if isNotFound(err) || isFolderNotFound(err) {
			vm.State = infrav1.VirtualMachineStateNotFound
			return vm, nil
		}
		return vm, err
	}

	//
	// At this point we know the VM exists, so it needs to be destroyed.
	//

	// Create a new virtualMachineContext to reconcile the VM.
	vmCtx := &virtualMachineContext{
		VMContext: *ctx,
		Obj:       object.NewVirtualMachine(ctx.Session.Client.Client, vmRef),
		Ref:       vmRef,
		State:     &vm,
	}

	// Power off the VM.
	powerState, err := vms.getPowerState(vmCtx)
	if err != nil {
		return vm, err
	}
	if powerState == infrav1.VirtualMachinePowerStatePoweredOn {
		task, err := vmCtx.Obj.PowerOff(ctx)
		if err != nil {
			return vm, err
		}
		ctx.VSphereVM.Status.TaskRef = task.Reference().Value
		if err = ctx.Patch(); err != nil {
			ctx.Logger.Error(err, "patch failed", "vm", ctx.String())
			return vm, err
		}
		ctx.Logger.Info("wait for VM to be powered off")
		return vm, nil
	}

	if ctx.ClusterModuleInfo != nil {
		provider := clustermodules.NewProvider(ctx.Session.TagManager.Client)
		err := provider.RemoveMoRefFromModule(ctx, *ctx.ClusterModuleInfo, vmCtx.Ref)
		if err != nil && !util.IsNotFoundError(err) {
			return vm, err
		}
		ctx.VSphereVM.Status.ModuleUUID = nil
	}

	// At this point the VM is not powered on and can be destroyed. Store the
	// destroy task's reference and return a requeue error.
	ctx.Logger.Info("destroying vm")
	task, err := vmCtx.Obj.Destroy(ctx)
	if err != nil {
		return vm, err
	}
	ctx.VSphereVM.Status.TaskRef = task.Reference().Value
	ctx.Logger.Info("wait for VM to be destroyed")
	return vm, nil
}

func (vms *VMService) reconcileNetworkStatus(ctx *virtualMachineContext) error {
	netStatus, err := vms.getNetworkStatus(ctx)
	if err != nil {
		return err
	}
	ctx.State.Network = netStatus
	return nil
}

// reconcileIPAddressClaims ensures that VSphereVMs that are configured with
// .spec.network.devices.addressFromPools have corresponding IPAddressClaims.
func (vms *VMService) reconcileIPAddressClaims(ctx *context.VMContext) (bool, error) {
	for devIdx, device := range ctx.VSphereVM.Spec.Network.Devices {
		for poolRefIdx, poolRef := range device.AddressesFromPools {
			ipAddrClaimName := IPAddressClaimName(ctx.VSphereVM.Name, devIdx, poolRefIdx)
			_, err := getIPAddrClaim(ctx, ipAddrClaimName)
			if err == nil {
				ctx.Logger.V(5).Info("IPAddressClaim found", "name", ipAddrClaimName)
			}
			if apierrors.IsNotFound(err) {
				if err = createIPAddressClaim(ctx, ipAddrClaimName, poolRef); err != nil {
					return false, err
				}
				msg := "Waiting for IPAddressClaim to have an IPAddress bound"
				markIPAddressClaimedConditionWaitingForClaimAddress(ctx.VSphereVM, msg)
			}
		}
	}
	return true, nil
}

// createIPAddressClaim sets up the ipam IPAddressClaim object and creates it in
// the API.
func createIPAddressClaim(ctx *context.VMContext, ipAddrClaimName string, poolRef corev1.TypedLocalObjectReference) error {
	ctx.Logger.Info("creating IPAddressClaim", "name", ipAddrClaimName)
	claim := &ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ipAddrClaimName,
			Namespace: ctx.VSphereVM.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: ctx.VSphereVM.APIVersion,
					Kind:       ctx.VSphereVM.Kind,
					Name:       ctx.VSphereVM.Name,
					UID:        ctx.VSphereVM.UID,
				},
			},
			Finalizers: []string{infrav1.IPAddressClaimFinalizer},
		},
		Spec: ipamv1.IPAddressClaimSpec{PoolRef: poolRef},
	}
	return ctx.Client.Create(ctx, claim)
}

// reconcileIPAddresses prevents successful reconcilliation of a VSphereVM
// until an IPAM Provider updates each IPAddressClaim associated to the
// VSphereVM with a reference to an IPAddress. This function is a no-op if the
// VSphereVM has no associated IPAddressClaims. A discovered IPAddress is
// expected to contain a valid IP, Prefix and Gateway.
func (vms *VMService) reconcileIPAddresses(ctx *virtualMachineContext) (bool, error) {
	ctx.IPAMState = map[string]infrav1.NetworkDeviceSpec{}

	ipamDeviceConfigs, err := buildIPAMDeviceConfigs(ctx)
	if err != nil {
		return false, err
	}

	var errs []error
	for _, ipamDeviceConfig := range ipamDeviceConfigs {
		var addressWithPrefixes []netip.Prefix
		for _, ipamAddress := range ipamDeviceConfig.IPAMAddresses {
			addressWithPrefix, err := parseAddressWithPrefix(ipamAddress)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if slices.Contains(addressWithPrefixes, addressWithPrefix) {
				errs = append(errs,
					fmt.Errorf("IPAddress %s/%s is a duplicate of another address: %q",
						ipamAddress.Namespace,
						ipamAddress.Name,
						addressWithPrefix))
				continue
			}

			gatewayAddr, err := parseGateway(ipamAddress, addressWithPrefix, ipamDeviceConfig)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if gatewayAddr.Is4() {
				ipamDeviceConfig.IPAMConfigGateway4 = ipamAddress.Spec.Gateway
			} else {
				ipamDeviceConfig.IPAMConfigGateway6 = ipamAddress.Spec.Gateway
			}

			addressWithPrefixes = append(addressWithPrefixes, addressWithPrefix)
		}

		if len(addressWithPrefixes) > 0 {
			ctx.IPAMState[ipamDeviceConfig.MACAddress] = infrav1.NetworkDeviceSpec{
				IPAddrs:  prefixesAsStrings(addressWithPrefixes),
				Gateway4: ipamDeviceConfig.IPAMConfigGateway4,
				Gateway6: ipamDeviceConfig.IPAMConfigGateway6,
			}
		}
	}

	if len(errs) > 0 {
		var msgs []string
		for _, err := range errs {
			msgs = append(msgs, err.Error())
		}
		msg := strings.Join(msgs, "\n")
		conditions.MarkFalse(ctx.VSphereVM,
			infrav1.IPAddressClaimedCondition,
			infrav1.IPAddressInvalidReason,
			clusterv1.ConditionSeverityError,
			msg)
		return false, errors.New(msg)
	}

	if len(ctx.IPAMState) > 0 {
		conditions.MarkTrue(ctx.VSphereVM, infrav1.IPAddressClaimedCondition)
	}

	return true, nil
}

// prefixesAsStrings converts []netip.Prefix to []string.
func prefixesAsStrings(prefixes []netip.Prefix) []string {
	prefixSrings := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		prefixSrings = append(prefixSrings, prefix.String())
	}
	return prefixSrings
}

// parseAddressWithPrefix converts a *ipamv1.IPAddress to a string, e.g. '10.0.0.1/24'.
func parseAddressWithPrefix(ipamAddress *ipamv1.IPAddress) (netip.Prefix, error) {
	addressWithPrefix := fmt.Sprintf("%s/%d", ipamAddress.Spec.Address, ipamAddress.Spec.Prefix)
	parsedPrefix, err := netip.ParsePrefix(addressWithPrefix)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("IPAddress %s/%s has invalid ip address: %q",
			ipamAddress.Namespace,
			ipamAddress.Name,
			addressWithPrefix,
		)
	}

	return parsedPrefix, nil
}

// parseGateway parses the gateway address on a ipamv1.IPAddress and ensures it
// does not conflict with the gateway addresses parsed from other
// ipamv1.IPAddresses on the current device. Gateway addresses must be the same
// family as the address on the ipamv1.IPAddress. Gateway addresses of one
// family must match the other addresses of the same family.
func parseGateway(ipamAddress *ipamv1.IPAddress, addressWithPrefix netip.Prefix, ipamDeviceConfig ipamDeviceConfig) (netip.Addr, error) {
	gatewayAddr, err := netip.ParseAddr(ipamAddress.Spec.Gateway)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("IPAddress %s/%s has invalid gateway: %q",
			ipamAddress.Namespace,
			ipamAddress.Name,
			ipamAddress.Spec.Gateway,
		)
	}

	if addressWithPrefix.Addr().Is4() != gatewayAddr.Is4() {
		return netip.Addr{}, fmt.Errorf("IPAddress %s/%s has mismatched gateway and address IP families",
			ipamAddress.Namespace,
			ipamAddress.Name,
		)
	}

	if gatewayAddr.Is4() {
		if areGatewaysMismatched(ipamDeviceConfig.NetworkSpecGateway4, ipamAddress.Spec.Gateway) {
			return netip.Addr{}, fmt.Errorf("the IPv4 Gateway for IPAddress %s does not match the Gateway4 already configured on device (index %d)",
				ipamAddress.Name,
				ipamDeviceConfig.DeviceIndex,
			)
		}
		if areGatewaysMismatched(ipamDeviceConfig.IPAMConfigGateway4, ipamAddress.Spec.Gateway) {
			return netip.Addr{}, fmt.Errorf("the IPv4 IPAddresses assigned to the same device (index %d) do not have the same gateway",
				ipamDeviceConfig.DeviceIndex,
			)
		}
	} else {
		if areGatewaysMismatched(ipamDeviceConfig.NetworkSpecGateway6, ipamAddress.Spec.Gateway) {
			return netip.Addr{}, fmt.Errorf("the IPv6 Gateway for IPAddress %s does not match the Gateway6 already configured on device (index %d)",
				ipamAddress.Name,
				ipamDeviceConfig.DeviceIndex,
			)
		}
		if areGatewaysMismatched(ipamDeviceConfig.IPAMConfigGateway6, ipamAddress.Spec.Gateway) {
			return netip.Addr{}, fmt.Errorf("the IPv6 IPAddresses assigned to the same device (index %d) do not have the same gateway",
				ipamDeviceConfig.DeviceIndex,
			)
		}
	}

	return gatewayAddr, nil
}

// buildIPAMDeviceConfigs checks that all the IPAddressClaims have been
// satisfied.
// If each IPAddressClaim has an associated IPAddress, a slice of
// ipamDeviceConfig is returned, one for each device with addressesFromPools.
// If any of the IPAddressClaims do not have an associated IPAddress yet,
// a false condition is set and an error is returned, effectively stopping the
// current reconcilliation loop.
func buildIPAMDeviceConfigs(ctx *virtualMachineContext) ([]ipamDeviceConfig, error) {
	boundClaims := 0
	totalClaims := 0
	ipamDeviceConfigs := []ipamDeviceConfig{}
	for devIdx, networkSpecDevice := range ctx.VSphereVM.Spec.Network.Devices {
		ipamDeviceConfig := ipamDeviceConfig{
			IPAMAddresses:       []*ipamv1.IPAddress{},
			MACAddress:          networkSpecDevice.MACAddr,
			NetworkSpecGateway4: networkSpecDevice.Gateway4,
			NetworkSpecGateway6: networkSpecDevice.Gateway6,
			DeviceIndex:         devIdx,
		}

		for poolRefIdx := range networkSpecDevice.AddressesFromPools {
			totalClaims++

			ipAddrClaimName := IPAddressClaimName(ctx.VSphereVM.Name, ipamDeviceConfig.DeviceIndex, poolRefIdx)

			ipAddrClaim, err := getIPAddrClaim(&ctx.VMContext, ipAddrClaimName)
			if err != nil {
				ctx.Logger.Error(err, "error fetching IPAddressClaim", "name", ipAddrClaimName)
				if apierrors.IsNotFound(err) {
					// it would be odd for this to occur, a findorcreate just happened in a previous step
					continue
				}
				return nil, err
			}

			ctx.Logger.V(5).Info("fetched IPAddressClaim", "name", ipAddrClaimName, "namespace", ctx.VSphereVM.Namespace)

			ipAddrName := ipAddrClaim.Status.AddressRef.Name
			if ipAddrName == "" {
				ctx.Logger.V(5).Info("IPAddress not yet bound to IPAddressClaim", "name", ipAddrClaimName, "namespace", ctx.VSphereVM.Namespace)
				continue
			}

			ipAddr := &ipamv1.IPAddress{}
			ipAddrKey := apitypes.NamespacedName{
				Namespace: ctx.VSphereVM.Namespace,
				Name:      ipAddrName,
			}

			if err := ctx.Client.Get(ctx, ipAddrKey, ipAddr); err != nil {
				// because the ref was set on the claim, it is expected this error should not occur
				return nil, err
			}

			ipamDeviceConfig.IPAMAddresses = append(ipamDeviceConfig.IPAMAddresses, ipAddr)
			boundClaims++
		}
		ipamDeviceConfigs = append(ipamDeviceConfigs, ipamDeviceConfig)
	}

	if boundClaims < totalClaims {
		msg := fmt.Sprintf("Waiting for IPAddressClaim to have an IPAddress bound, %d out of %d bound", boundClaims, totalClaims)
		markIPAddressClaimedConditionWaitingForClaimAddress(ctx.VSphereVM, msg)
		return nil, errors.New(msg)
	}

	return ipamDeviceConfigs, nil
}

// areGatewaysMismatched checks that a gateway for a device is equal to an
// IPAddresses gateway. We can assume that IPAddresses will always have
// gateways so we do not need to check for empty string. It is possible to
// configure a device and not a gateway, we don't want to fail in that case.
func areGatewaysMismatched(deviceGateway, ipAddressGateway string) bool {
	return deviceGateway != "" && deviceGateway != ipAddressGateway
}

// getIPAddrClaim fetches an IPAddressClaim from the api with the given name.
func getIPAddrClaim(ctx *context.VMContext, ipAddrClaimName string) (*ipamv1.IPAddressClaim, error) {
	ipAddrClaim := &ipamv1.IPAddressClaim{}
	ipAddrClaimKey := apitypes.NamespacedName{
		Namespace: ctx.VSphereVM.Namespace,
		Name:      ipAddrClaimName,
	}

	ctx.Logger.V(5).Info("fetching IPAddressClaim", "name", ipAddrClaimKey.String())
	if err := ctx.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim); err != nil {
		return nil, err
	}
	return ipAddrClaim, nil
}

func (vms *VMService) reconcileMetadata(ctx *virtualMachineContext) (bool, error) {
	existingMetadata, err := vms.getMetadata(ctx)
	if err != nil {
		return false, err
	}

	newMetadata, err := util.GetMachineMetadata(ctx.VSphereVM.Name, *ctx.VSphereVM, ctx.IPAMState, ctx.State.Network...)
	if err != nil {
		return false, err
	}

	// If the metadata is the same then return early.
	if string(newMetadata) == existingMetadata {
		return true, nil
	}

	ctx.Logger.Info("updating metadata")
	taskRef, err := vms.setMetadata(ctx, newMetadata)
	if err != nil {
		return false, errors.Wrapf(err, "unable to set metadata on vm %s", ctx)
	}

	ctx.VSphereVM.Status.TaskRef = taskRef
	ctx.Logger.Info("wait for VM metadata to be updated")
	return false, nil
}

func (vms *VMService) reconcilePowerState(ctx *virtualMachineContext) (bool, error) {
	powerState, err := vms.getPowerState(ctx)
	if err != nil {
		return false, err
	}
	switch powerState {
	case infrav1.VirtualMachinePowerStatePoweredOff:
		ctx.Logger.Info("powering on")
		task, err := ctx.Obj.PowerOn(ctx)
		if err != nil {
			conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, infrav1.PoweringOnFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			return false, errors.Wrapf(err, "failed to trigger power on op for vm %s", ctx)
		}
		conditions.MarkFalse(ctx.VSphereVM, infrav1.VMProvisionedCondition, infrav1.PoweringOnReason, clusterv1.ConditionSeverityInfo, "")

		// Update the VSphereVM.Status.TaskRef to track the power-on task.
		ctx.VSphereVM.Status.TaskRef = task.Reference().Value
		if err = ctx.Patch(); err != nil {
			ctx.Logger.Error(err, "patch failed", "vm", ctx.String())
			return false, err
		}

		// Once the VM is successfully powered on, a reconcile request should be
		// triggered once the VM reports IP addresses are available.
		reconcileVSphereVMWhenNetworkIsReady(ctx, task)

		ctx.Logger.Info("wait for VM to be powered on")
		return false, nil
	case infrav1.VirtualMachinePowerStatePoweredOn:
		ctx.Logger.Info("powered on")
		return true, nil
	default:
		return false, errors.Errorf("unexpected power state %q for vm %s", powerState, ctx)
	}
}

func (vms *VMService) reconcileStoragePolicy(ctx *virtualMachineContext) error {
	if ctx.VSphereVM.Spec.StoragePolicyName == "" {
		ctx.Logger.V(5).Info("storage policy not defined. skipping reconcile storage policy")
		return nil
	}

	// return early if the VM is already powered on
	powerState, err := vms.getPowerState(ctx)
	if err != nil {
		return err
	}
	if powerState == infrav1.VirtualMachinePowerStatePoweredOn {
		ctx.Logger.Info("VM powered on. skipping reconcile storage policy")
		return nil
	}

	pbmClient, err := pbm.NewClient(ctx, ctx.Session.Client.Client)
	if err != nil {
		return errors.Wrap(err, "unable to create pbm client")
	}
	storageProfileID, err := pbmClient.ProfileIDByName(ctx, ctx.VSphereVM.Spec.StoragePolicyName)
	if err != nil {
		return errors.Wrap(err, "unable to retrieve storage profile ID")
	}
	entities, err := pbmClient.QueryAssociatedEntity(ctx, pbmTypes.PbmProfileId{UniqueId: storageProfileID}, "virtualDiskId")
	if err != nil {
		return err
	}

	var changes []types.BaseVirtualDeviceConfigSpec
	devices, err := ctx.Obj.Device(ctx)
	if err != nil {
		return err
	}

	disks := devices.SelectByType((*types.VirtualDisk)(nil))
	for _, d := range disks {
		disk := d.(*types.VirtualDisk) //nolint:forcetypeassert
		found := false
		// entities associated with storage policy has key in the form <vm-ID>:<disk>
		diskID := fmt.Sprintf("%s:%d", ctx.Obj.Reference().Value, disk.Key)
		for _, e := range entities {
			if e.Key == diskID {
				found = true
				break
			}
		}

		if !found {
			// disk wasn't associated with storage policy, create a device change to make the association
			config := &types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationEdit,
				Device:    disk,
				Profile: []types.BaseVirtualMachineProfileSpec{
					&types.VirtualMachineDefinedProfileSpec{ProfileId: storageProfileID},
				},
			}
			changes = append(changes, config)
		}
	}

	if len(changes) > 0 {
		task, err := ctx.Obj.Reconfigure(ctx, types.VirtualMachineConfigSpec{
			VmProfile: []types.BaseVirtualMachineProfileSpec{
				&types.VirtualMachineDefinedProfileSpec{ProfileId: storageProfileID},
			},
			DeviceChange: changes,
		})
		if err != nil {
			return errors.Wrapf(err, "unable to set storagePolicy on vm %s", ctx)
		}
		ctx.VSphereVM.Status.TaskRef = task.Reference().Value
	}
	return nil
}

func (vms *VMService) reconcileUUID(ctx *virtualMachineContext) {
	ctx.State.BiosUUID = ctx.Obj.UUID(ctx)
}

func (vms *VMService) reconcileHardwareVersion(ctx *virtualMachineContext) (bool, error) {
	if ctx.VSphereVM.Spec.HardwareVersion != "" {
		var virtualMachine mo.VirtualMachine
		if err := ctx.Obj.Properties(ctx, ctx.Obj.Reference(), []string{"config.version"}, &virtualMachine); err != nil {
			return false, errors.Wrapf(err, "error getting guestInfo version information from VM %s", ctx.VSphereVM.Name)
		}
		toUpgrade, err := util.LessThan(virtualMachine.Config.Version, ctx.VSphereVM.Spec.HardwareVersion)
		if err != nil {
			return false, errors.Wrapf(err, "failed to parse hardware version")
		}
		if toUpgrade {
			ctx.Logger.Info("upgrading hardware version",
				"from", virtualMachine.Config.Version,
				"to", ctx.VSphereVM.Spec.HardwareVersion)
			task, err := ctx.Obj.UpgradeVM(ctx, ctx.VSphereVM.Spec.HardwareVersion)
			if err != nil {
				return false, errors.Wrapf(err, "error trigging upgrade op for machine %s", ctx)
			}
			ctx.VSphereVM.Status.TaskRef = task.Reference().Value
			return false, nil
		}
	}
	return true, nil
}

func (vms *VMService) reconcilePCIDevices(ctx *virtualMachineContext) error {
	if expectedPciDevices := ctx.VSphereVM.Spec.VirtualMachineCloneSpec.PciDevices; len(expectedPciDevices) != 0 {
		specsToBeAdded, err := pci.CalculateDevicesToBeAdded(ctx, ctx.Obj, expectedPciDevices)
		if err != nil {
			return err
		}

		if len(specsToBeAdded) == 0 {
			if conditions.Has(ctx.VSphereVM, infrav1.PCIDevicesDetachedCondition) {
				conditions.Delete(ctx.VSphereVM, infrav1.PCIDevicesDetachedCondition)
			}
			ctx.Logger.V(5).Info("no new PCI devices to be added")
			return nil
		}

		powerState, err := ctx.Obj.PowerState(ctx)
		if err != nil {
			return err
		}
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			// This would arise only when the PCI device is manually removed from
			// the VM post creation.
			ctx.Logger.Info("PCI device cannot be attached in powered on state")
			conditions.MarkFalse(ctx.VSphereVM,
				infrav1.PCIDevicesDetachedCondition,
				infrav1.NotFoundReason,
				clusterv1.ConditionSeverityWarning,
				"PCI devices removed after VM was powered on")
			return errors.Errorf("missing PCI devices")
		}
		ctx.Logger.Info("PCI devices to be added", "number", len(specsToBeAdded))
		if err := ctx.Obj.AddDevice(ctx, pci.ConstructDeviceSpecs(specsToBeAdded)...); err != nil {
			return errors.Wrapf(err, "error adding pci devices for %q", ctx)
		}
	}
	return nil
}

func (vms *VMService) getPowerState(ctx *virtualMachineContext) (infrav1.VirtualMachinePowerState, error) {
	powerState, err := ctx.Obj.PowerState(ctx)
	if err != nil {
		return "", err
	}

	switch powerState {
	case types.VirtualMachinePowerStatePoweredOn:
		return infrav1.VirtualMachinePowerStatePoweredOn, nil
	case types.VirtualMachinePowerStatePoweredOff:
		return infrav1.VirtualMachinePowerStatePoweredOff, nil
	case types.VirtualMachinePowerStateSuspended:
		return infrav1.VirtualMachinePowerStateSuspended, nil
	default:
		return "", errors.Errorf("unexpected power state %q for vm %s", powerState, ctx)
	}
}

func (vms *VMService) getMetadata(ctx *virtualMachineContext) (string, error) {
	var (
		obj mo.VirtualMachine

		pc    = property.DefaultCollector(ctx.Session.Client.Client)
		props = []string{"config.extraConfig"}
	)

	if err := pc.RetrieveOne(ctx, ctx.Ref, props, &obj); err != nil {
		return "", errors.Wrapf(err, "unable to fetch props %v for vm %s", props, ctx)
	}
	if obj.Config == nil {
		return "", nil
	}

	var metadataBase64 string
	for _, ec := range obj.Config.ExtraConfig {
		if optVal := ec.GetOptionValue(); optVal != nil {
			// TODO(akutz) Using a switch instead of if in case we ever
			//             want to check the metadata encoding as well.
			//             Since the image stamped images always use
			//             base64, it should be okay to not check.
			//nolint:gocritic
			switch optVal.Key {
			case guestInfoKeyMetadata:
				if v, ok := optVal.Value.(string); ok {
					metadataBase64 = v
				}
			}
		}
	}

	if metadataBase64 == "" {
		return "", nil
	}

	metadataBuf, err := base64.StdEncoding.DecodeString(metadataBase64)
	if err != nil {
		return "", errors.Wrapf(err, "unable to decode metadata for %s", ctx)
	}

	return string(metadataBuf), nil
}

func (vms *VMService) reconcileHostInfo(ctx *virtualMachineContext) error {
	host, err := ctx.Obj.HostSystem(ctx)
	if err != nil {
		return err
	}
	name, err := host.ObjectName(ctx)
	if err != nil {
		return err
	}
	ctx.VSphereVM.Status.Host = name
	return nil
}

func (vms *VMService) setMetadata(ctx *virtualMachineContext, metadata []byte) (string, error) {
	var extraConfig extra.Config

	extraConfig.SetCloudInitMetadata(metadata)

	task, err := ctx.Obj.Reconfigure(ctx, types.VirtualMachineConfigSpec{
		ExtraConfig: extraConfig,
	})
	if err != nil {
		return "", errors.Wrapf(err, "unable to set metadata on vm %s", ctx)
	}

	return task.Reference().Value, nil
}

func (vms *VMService) getNetworkStatus(ctx *virtualMachineContext) ([]infrav1.NetworkStatus, error) {
	allNetStatus, err := govmominet.GetNetworkStatus(ctx, ctx.Session.Client.Client, ctx.Ref)
	if err != nil {
		return nil, err
	}
	ctx.Logger.V(4).Info("got allNetStatus", "status", allNetStatus)
	apiNetStatus := []infrav1.NetworkStatus{}
	for _, s := range allNetStatus {
		apiNetStatus = append(apiNetStatus, infrav1.NetworkStatus{
			Connected:   s.Connected,
			IPAddrs:     sanitizeIPAddrs(&ctx.VMContext, s.IPAddrs),
			MACAddr:     s.MACAddr,
			NetworkName: s.NetworkName,
		})
	}
	return apiNetStatus, nil
}

// getBootstrapData obtains a machine's bootstrap data from the relevant k8s secret and returns the
// data and its format.
func (vms *VMService) getBootstrapData(ctx *context.VMContext) ([]byte, bootstrapv1.Format, error) {
	if ctx.VSphereVM.Spec.BootstrapRef == nil {
		ctx.Logger.Info("VM has no bootstrap data")
		return nil, "", nil
	}

	secret := &corev1.Secret{}
	secretKey := apitypes.NamespacedName{
		Namespace: ctx.VSphereVM.Spec.BootstrapRef.Namespace,
		Name:      ctx.VSphereVM.Spec.BootstrapRef.Name,
	}
	if err := ctx.Client.Get(ctx, secretKey, secret); err != nil {
		return nil, "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for %s", ctx)
	}

	format, ok := secret.Data["format"]
	if !ok || len(format) == 0 {
		// Bootstrap data format is missing or empty - assume cloud-config.
		format = []byte(bootstrapv1.CloudConfig)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, bootstrapv1.Format(format), nil
}

func (vms *VMService) reconcileVMGroupInfo(ctx *virtualMachineContext) (bool, error) {
	if ctx.VSphereFailureDomain == nil || ctx.VSphereFailureDomain.Spec.Topology.Hosts == nil {
		ctx.Logger.V(5).Info("hosts topology in failure domain not defined. skipping reconcile VM group")
		return true, nil
	}

	topology := ctx.VSphereFailureDomain.Spec.Topology
	vmGroup, err := cluster.FindVMGroup(ctx, *topology.ComputeCluster, topology.Hosts.VMGroupName)
	if err != nil {
		return false, errors.Wrapf(err, "unable to find VM Group %s", topology.Hosts.VMGroupName)
	}

	hasVM, err := vmGroup.HasVM(ctx.Ref)
	if err != nil {
		return false, errors.Wrapf(err, "unable to find VM Group %s membership", topology.Hosts.VMGroupName)
	}

	if !hasVM {
		task, err := vmGroup.Add(ctx, ctx.Ref)
		if err != nil {
			return false, errors.Wrapf(err, "failed to add VM %s to VM group", ctx.VSphereVM.Name)
		}
		ctx.VSphereVM.Status.TaskRef = task.Reference().Value
		ctx.Logger.Info("wait for VM to be added to group")
		return false, nil
	}
	return true, nil
}

func (vms *VMService) reconcileTags(ctx *virtualMachineContext) error {
	if len(ctx.VSphereVM.Spec.TagIDs) == 0 {
		ctx.Logger.V(5).Info("no tags defined. skipping tags reconciliation")
		return nil
	}

	err := ctx.Session.TagManager.AttachMultipleTagsToObject(ctx, ctx.VSphereVM.Spec.TagIDs, ctx.Ref)
	if err != nil {
		return errors.Wrapf(err, "failed to attach tags %v to VM %s", ctx.VSphereVM.Spec.TagIDs, ctx.VSphereVM.Name)
	}

	return nil
}

func (vms *VMService) reconcileClusterModuleMembership(ctx *virtualMachineContext) error {
	if ctx.ClusterModuleInfo != nil {
		ctx.Logger.V(5).Info("add vm to module", "moduleUUID", *ctx.ClusterModuleInfo)
		provider := clustermodules.NewProvider(ctx.Session.TagManager.Client)

		if err := provider.AddMoRefToModule(ctx, *ctx.ClusterModuleInfo, ctx.Ref); err != nil {
			return err
		}
		ctx.VSphereVM.Status.ModuleUUID = ctx.ClusterModuleInfo
	}
	return nil
}

func markIPAddressClaimedConditionWaitingForClaimAddress(vm *infrav1.VSphereVM, msg string) {
	conditions.MarkFalse(vm,
		infrav1.IPAddressClaimedCondition,
		infrav1.WaitingForIPAddressReason,
		clusterv1.ConditionSeverityInfo,
		msg)
}

// IPAddressClaimName returns a name given a VsphereVM name, deviceIndex, and
// poolIndex.
func IPAddressClaimName(vmName string, deviceIndex, poolIndex int) string {
	return fmt.Sprintf("%s-%d-%d", vmName, deviceIndex, poolIndex)
}
