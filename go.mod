module sigs.k8s.io/cluster-api-provider-vsphere

go 1.16

require (
	github.com/antihax/optional v1.0.0
	github.com/go-logr/logr v1.2.0
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.2.0
	github.com/hashicorp/go-version v1.3.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.2.1
	github.com/vmware-tanzu/net-operator-api v0.0.0-20210401185409-b0dc6c297707
	github.com/vmware-tanzu/vm-operator-api v0.1.4-0.20211029224930-6ec913d11bff
	github.com/vmware-tanzu/vm-operator/external/ncp v0.0.0-20211209213435-0f4ab286f64f
	github.com/vmware-tanzu/vm-operator/external/tanzu-topology v0.0.0-20211209213435-0f4ab286f64f
	github.com/vmware/govmomi v0.27.1
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/cluster-bootstrap v0.23.0
	k8s.io/component-base v0.23.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.0
	sigs.k8s.io/cluster-api/test v1.0.0
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega => github.com/onsi/gomega v1.11.0
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.1.0
	sigs.k8s.io/cluster-api/test => sigs.k8s.io/cluster-api/test v1.1.0
)
