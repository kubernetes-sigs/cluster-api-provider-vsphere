module sigs.k8s.io/cluster-api-provider-vsphere

go 1.16

require (
	github.com/antihax/optional v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/vmware/govmomi v0.23.1
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/gcfg.v1 v1.2.3
	k8s.io/api v0.21.0-beta.1
	k8s.io/apiextensions-apiserver v0.21.0-beta.1
	k8s.io/apimachinery v0.21.0-beta.1
	k8s.io/client-go v0.21.0-beta.1
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/cluster-api v0.3.11-0.20210324181938-d2a9e7c9fb2e
	sigs.k8s.io/controller-runtime v0.9.0-alpha.1
	sigs.k8s.io/kind v0.9.0
	sigs.k8s.io/yaml v1.2.0
)
