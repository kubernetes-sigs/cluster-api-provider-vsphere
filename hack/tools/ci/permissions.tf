resource vsphere_role "capv-ci-content-library" {
  name = "capv-ci-content-library"
  role_privileges = [
    "ContentLibrary.DownloadSession",
    "ContentLibrary.ReadStorage",
    "ContentLibrary.SyncLibraryItem",
  ]
}

resource vsphere_role "capv-ci" {
  name = "capv-ci"
  role_privileges = [
    "Cns.Searchable",
    "Datastore.AllocateSpace",
    "Datastore.Browse",
    "Datastore.FileManagement",
    "Folder.Create",
    "Folder.Delete",
    "Global.SetCustomField",
    "Network.Assign",
    "Resource.AssignVMToPool",
    "Resource.CreatePool",
    "Resource.DeletePool",
    "Sessions.GlobalMessage",
    "Sessions.ValidateSession",
    "StorageProfile.View",
    "VApp.ApplicationConfig",
    "VApp.Import",
    "VApp.InstanceConfig",
    "VirtualMachine.Config.AddExistingDisk",
    "VirtualMachine.Config.AddNewDisk",
    "VirtualMachine.Config.AddRemoveDevice",
    "VirtualMachine.Config.AdvancedConfig",
    "VirtualMachine.Config.Annotation",
    "VirtualMachine.Config.CPUCount",
    "VirtualMachine.Config.ChangeTracking",
    "VirtualMachine.Config.DiskExtend",
    "VirtualMachine.Config.EditDevice",
    "VirtualMachine.Config.HostUSBDevice",
    "VirtualMachine.Config.ManagedBy",
    "VirtualMachine.Config.Memory",
    "VirtualMachine.Config.RawDevice",
    "VirtualMachine.Config.RemoveDisk",
    "VirtualMachine.Config.Resource",
    "VirtualMachine.Config.Settings",
    "VirtualMachine.Config.SwapPlacement",
    "VirtualMachine.Config.UpgradeVirtualHardware",
    "VirtualMachine.Interact.ConsoleInteract",
    "VirtualMachine.Interact.DeviceConnection",
    "VirtualMachine.Interact.PowerOff",
    "VirtualMachine.Interact.PowerOn",
    "VirtualMachine.Interact.SetCDMedia",
    "VirtualMachine.Interact.SetFloppyMedia",
    "VirtualMachine.Inventory.Create",
    "VirtualMachine.Inventory.CreateFromExisting",
    "VirtualMachine.Inventory.Delete",
    "VirtualMachine.Provisioning.Clone",
    "VirtualMachine.Provisioning.CloneTemplate",
    "VirtualMachine.Provisioning.CreateTemplateFromVM",
    "VirtualMachine.Provisioning.DeployTemplate",
    "VirtualMachine.Provisioning.DiskRandomRead",
    "VirtualMachine.Provisioning.GetVmFiles",
    "VirtualMachine.State.CreateSnapshot",
    "VirtualMachine.State.RemoveSnapshot",
  ]
}

# resource "vsphere_entity_permissions" "rp-capv" {
#   entity_id = vsphere_resource_pool.capi.id
#   entity_type = "ResourcePool"
#   permissions {
#     user_or_group = "vsphere.local\\DCClients"
#     propagate = true
#     is_group = true
#     role_id = data.vsphere_role.role1.id
#   }
#   permissions {
#     user_or_group = "vsphere.local\\ExternalIDPUsers"
#     propagate = true
#     is_group = true
#     role_id = vsphere_role.role2.id
#   }
# }
