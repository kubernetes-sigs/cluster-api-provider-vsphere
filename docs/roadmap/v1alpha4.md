# v1alpha4 roadmap

## Breaking API changes

* remove the CSI fields from the `VSphereClusterSpec` in favour of CRS
* remove the CPI fields from the `VSphereClusterSpec` in favour of CRS
* move away from `ObjectRefence`
* remove the HAProxy support (`LoadBalancerRef`, `HAProxyLoadBalancer` etc..)
* introduce `Failure Domains` support
* remove the `insecure` field in favour of the `thumbprint` field
* remove `PreferredAPIServerCIDR`
