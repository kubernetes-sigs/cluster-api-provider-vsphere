module sigs.k8s.io/cluster-api-provider-vsphere

go 1.12

require (
	github.com/antihax/optional v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.3.1
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/pkg/errors v0.8.1
	github.com/vmware/govmomi v0.21.0
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.0.0-20191121015604-11707872ac1c
	k8s.io/apimachinery v0.0.0-20191121015412-41065c7a8c2a
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
	sigs.k8s.io/cluster-api v0.2.6-0.20200125173702-54f26d7fd2b5
	sigs.k8s.io/cluster-api/test/framework v0.0.0-20200125173702-54f26d7fd2b5
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/yaml v1.1.0
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.2.6-0.20200125173702-54f26d7fd2b5
)
