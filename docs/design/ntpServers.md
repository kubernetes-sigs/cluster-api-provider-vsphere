## Use Case
For a distributed system to operate properly, you need the time to be synchronized across the machines. For instance, when a master node is created kubeadm generates a bunch of certificates for the system to use for various purposes. Now in case, the time of the master node and the worker nodes that come up later is not the same there is a possibility that the certificates generated would not work. This is just a simple example of why time synchronization across the nodes is important. A simple way to ensure that is by synchronizing the time on all the machines with NTP server/s.

## How to use
Provide the list of ntp servers accessible in your environment via the `ntpServers` property as shown below
```
providerSpec:
  value:
    apiVersion: "vsphereproviderconfig/v1alpha1"
    kind: "VsphereMachineProviderConfig"
    machineSpec:
      ...
      ntpServers:
      - 0.pool.ntp.org
      - 1.pool.ntp.org
```
