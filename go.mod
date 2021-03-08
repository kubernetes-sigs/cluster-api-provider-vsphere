module sigs.k8s.io/cluster-api-provider-vsphere

go 1.15

require (
	github.com/antihax/optional v1.0.0
	github.com/go-logr/logr v0.3.0
	github.com/google/go-cmp v0.5.2
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	github.com/vmware/govmomi v0.23.1
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/cluster-api v0.3.11-0.20210302171319-f7351b165992
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/kind v0.9.0
	sigs.k8s.io/yaml v1.2.0
)
