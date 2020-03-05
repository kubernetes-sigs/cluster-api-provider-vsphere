module sigs.k8s.io/cluster-api-provider-vsphere

go 1.12

require (
	github.com/antihax/optional v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.3.1
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.8.1
	github.com/pkg/errors v0.9.0
	github.com/spf13/cobra v0.0.5
	github.com/vmware/govmomi v0.21.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898 // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	sigs.k8s.io/cluster-api v0.3.0-rc.3
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/yaml v1.1.0
)
