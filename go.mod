module sigs.k8s.io/cluster-api-provider-vsphere

go 1.16

require (
	github.com/antihax/optional v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.2.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.2.1
	github.com/vmware/govmomi v0.23.1
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/component-base v0.22.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/cluster-api v0.4.4-0.20210924162544-f2cc2113db1e
	sigs.k8s.io/cluster-api/test v0.4.4-0.20210924162544-f2cc2113db1e // TODO: update to CAPI v1beta1 release
	sigs.k8s.io/controller-runtime v0.10.1
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega => github.com/onsi/gomega v1.11.0
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.4-0.20210924162544-f2cc2113db1e
)
