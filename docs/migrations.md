# v1alpha3 HAProxy to v1alpha3 kube-vip

`HAProxyLoadBalancer` is targeted for removal in `v1alpha4`, to migrate from an existing `HAProxyLoadBalancer` to `kube-vip` you will need the following steps:

1 - get the HAProxyLoadBalancer existing IP with `kubectl get haproxyloadbalancer CLUSTER_NAME -n NAMESPACE -o template='{{.status.address}}'`
2 - in following patch `patch.yaml` file, replace the `vip_address` value with the IP of the `HAProxyLoadBalancer`:

```yaml
spec:
  kubeadmConfigSpec:
    files:
    - content: |
        apiVersion: v1
        kind: Pod
        metadata:
          creationTimestamp: null
          name: kube-vip
          namespace: kube-system
        spec:
          containers:
          - args:
            - start
            env:
            - name: vip_arp
              value: "true"
            - name: vip_leaderelection
              value: "true"
            - name: vip_address
              value: <HAPROXY_IP>
            - name: vip_interface
              value: eth0
            - name: vip_leaseduration
              value: "15"
            - name: vip_renewdeadline
              value: "10"
            - name: vip_retryperiod
              value: "2"
            image: plndr/kube-vip:0.2.0
            imagePullPolicy: IfNotPresent
            name: kube-vip
            resources: {}
            securityContext:
              capabilities:
                add:
                - NET_ADMIN
                - SYS_TIME
            volumeMounts:
            - mountPath: /etc/kubernetes/admin.conf
              name: kubeconfig
          hostNetwork: true
          volumes:
          - hostPath:
              path: /etc/kubernetes/admin.conf
              type: FileOrCreate
            name: kubeconfig
        status: {}
      owner: root:root
      path: /etc/kubernetes/manifests/kube-vip.yaml
```

3 - patch `KubeadmControlPlane` with the following command `kubectl patch kcp KCP_NAME -n NAMESPACE --type merge  --patch "$(cat patch.yaml)"` replace `KCP_NAME` and `NAMESPACE` with the `KubeadmControlPlane` name and namespace
NOTE: this patch will override any already existing files you have in `.spec.Files`, if you want to preserve these you will need to include as part of `patch.yaml`

4 - wait until the first control plane machine with the new spec is in a `Running` state

5 -  remove `HAProxyLoadBalancer` with `kubectl delete haproxyloadbalancer HAPROXY_NAME -n NAMESPACE`

6 -  remove the `loadBalancerRef` from the `vsphereCluster` object (e.g. `kubectl edit vspherecluster CLUSTER_NAME`)

7 - once the rollout of the new machines is finished, you will need to make a static reservation for the control plane endpoint IP at the DHCP server-level (if you're using DHCP)
