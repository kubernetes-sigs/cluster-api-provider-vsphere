# Integration tests

The [crds](./crds/) are copied from the vm-operators version which is consumed as go module.

These should get updated when bumping the vm-operator dependency.

To sync the new CRD's use the following script **and** update `kustomization.yaml` accordingly.

```sh
make clean-vm-operator checkout-vm-operator
rm -r config/deployments/integration-tests/crds
cp -r test/infrastructure/vm-operator/vm-operator.tmp/config/crd/bases config/deployments/integration-tests/crds
# Note: for now we only need the AvailabilityZone CRD in our integration tests
cp test/infrastructure/vm-operator/vm-operator.tmp/config/crd/external-crds/topology.tanzu.vmware.com_availabilityzones.yaml config/deployments/integration-tests/crds

make clean-vm-operator
```
