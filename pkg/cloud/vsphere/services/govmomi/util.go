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

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/govmomi/net"
)

func findVM(ctx *context.MachineContext) (*object.VirtualMachine, error) {
	// If no MachineRef is present then lookup the VM by its UUID.
	if ctx.MachineConfig.MachineRef == "" {
		vmRef, err := findVMByInstanceUUID(ctx)
		if err != nil {
			return nil, err
		}
		ctx.MachineConfig.MachineRef = vmRef
	}

	// If there is a MachineRef defined then use it to determine if the machine
	// exists.
	if ctx.MachineConfig.MachineRef != "" {
		return getVM(ctx), nil
	}

	return nil, nil
}

func findVMByInstanceUUID(ctx *context.MachineContext) (string, error) {
	ctx.Logger.V(6).Info("finding vm by instance UUID", "instance-uuid", ctx.Machine.UID)
	ref, err := ctx.Session.FindByInstanceUUID(ctx, string(ctx.Machine.UID))
	if err != nil {
		return "", err
	}
	if ref != nil {
		ctx.Logger.V(6).Info("found vm by instance UUID", "instance-uuid", ctx.Machine.UID)
		return ref.Reference().Value, nil
	}
	return "", nil
}

func getVM(ctx *context.MachineContext) *object.VirtualMachine {
	moRef := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: ctx.MachineConfig.MachineRef,
	}
	var obj mo.VirtualMachine
	if err := ctx.Session.RetrieveOne(ctx, moRef, []string{"name"}, &obj); err != nil {
		return nil
	}
	return object.NewVirtualMachine(ctx.Session.Client.Client, moRef)
}

func getTask(ctx *context.MachineContext) *mo.Task {
	var obj mo.Task
	moRef := types.ManagedObjectReference{
		Type:  morefTypeTask,
		Value: ctx.MachineStatus.TaskRef,
	}
	if err := ctx.Session.RetrieveOne(ctx, moRef, []string{"info"}, &obj); err != nil {
		return nil
	}
	return &obj
}

// getNetworkStatus returns the network status for a machine. The order matches
// the order of MachineConfig.MachineSpec.Network.Devices.
func getNetworkStatus(ctx *context.MachineContext) ([]v1alpha1.NetworkStatus, error) {
	allNetStatus, err := net.GetNetworkStatus(ctx, ctx.Session.Client.Client, *(ctx.GetMoRef()))
	if err != nil {
		return nil, err
	}
	ctx.Logger.V(6).Info("got allNetStatus", "status", allNetStatus)
	apiNetStatus := []v1alpha1.NetworkStatus{}
	for _, s := range allNetStatus {
		apiNetStatus = append(apiNetStatus, v1alpha1.NetworkStatus{
			Connected:   s.Connected,
			IPAddrs:     sanitizeIPAddrs(ctx, s.IPAddrs),
			MACAddr:     s.MACAddr,
			NetworkName: s.NetworkName,
		})
	}
	return apiNetStatus, nil
}

func sanitizeIPAddrs(ctx *context.MachineContext, ipAddrs []string) []string {
	if len(ipAddrs) == 0 {
		return nil
	}
	newIPAddrs := []string{}
	for _, addr := range ipAddrs {
		if err := net.ErrOnLocalOnlyIPAddr(addr); err != nil {
			ctx.Logger.V(8).Info("ignoring IP address", "reason", err)
		} else {
			newIPAddrs = append(newIPAddrs, addr)
		}
	}
	return newIPAddrs
}

func getExistingMetadata(ctx *context.MachineContext) (string, error) {
	var (
		obj mo.VirtualMachine

		moRef = *(ctx.GetMoRef())
		pc    = property.DefaultCollector(ctx.Session.Client.Client)
		props = []string{"config.extraConfig"}
	)

	if err := pc.RetrieveOne(ctx, moRef, props, &obj); err != nil {
		return "", errors.Wrapf(err, "unable to fetch props %v for vm %v", props, moRef)
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
		return "", errors.Wrapf(err, "unable to decode metadata for %q", ctx)
	}

	return string(metadataBuf), nil
}
