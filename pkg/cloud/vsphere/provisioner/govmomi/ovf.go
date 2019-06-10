package govmomi

import (
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
)

func (pv *Provisioner) addOvfEnv(machine *GovmomiMachine, machineConfig *vsphereconfigv1.VsphereMachineProviderConfig, vmProps *mo.VirtualMachine, userData string, metaData string) []types.BaseOptionValue {
	// OVF Environment
	// build up Environment in order to marshal to xml

	var opts []types.BaseOptionValue

	if machineConfig.MachineSpec.VsphereCloudInit {
		// In case of vsphere cloud-init datasource present, set the appropriate extraconfig options
		opts = append(opts, &types.OptionValue{Key: "guestinfo.metadata", Value: metaData})
		opts = append(opts, &types.OptionValue{Key: "guestinfo.metadata.encoding", Value: "base64"})
		opts = append(opts, &types.OptionValue{Key: "guestinfo.userdata", Value: userData})
		opts = append(opts, &types.OptionValue{Key: "guestinfo.userdata.encoding", Value: "base64"})
	} else {
		// This case is to support backwards compatibility, where we are using the ubuntu cloud image ovf properties
		// to drive the cloud-init workflow. Once the vsphere cloud-init datastore is merged as part of the official
		// cloud-init, then we can potentially remove this flag from the spec as then all the native cloud images
		// available for the different distros will include this new datasource.
		// See (https://github.com/akutz/cloud-init-vmware-guestinfo/ - vmware cloud-init datasource) for details
		var props []ovf.EnvProperty
		sshKey, _ := pv.GetSSHPublicKey(machine.cluster)
		props = append(props, ovf.EnvProperty{
			Key:   "user-data",
			Value: userData,
		},
			ovf.EnvProperty{
				Key:   "public-keys",
				Value: sshKey,
			},
			ovf.EnvProperty{
				Key:   "hostname",
				Value: machine.Name,
			})
		a := machine.s.session.ServiceContent.About
		env := ovf.Env{
			EsxID: vmProps.Reference().Value,
			Platform: &ovf.PlatformSection{
				Kind:    a.Name,
				Version: a.Version,
				Vendor:  a.Vendor,
				Locale:  "US",
			},
			Property: &ovf.PropertySection{
				Properties: props,
			},
		}

		opts = append(opts, &types.OptionValue{
			Key:   "guestinfo.ovfEnv",
			Value: env.MarshalManual(),
		})
	}
	return opts
}
