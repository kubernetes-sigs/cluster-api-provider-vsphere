module sigs.k8s.io/cluster-api-provider-vsphere

go 1.12

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20181213151034-8d9ed539ba31

replace k8s.io/api => k8s.io/api v0.0.0-20181213150558-05914d821849

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20181213153335-0fe22c71c476

replace sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.0.0-20180825012133-999adc0c9bd4

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.1.10

require (
	github.com/Azure/go-autorest/autorest v0.2.0 // indirect
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/evanphx/json-patch v4.2.0+incompatible // indirect
	github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gophercloud/gophercloud v0.1.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/onsi/gomega v1.5.0
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/prometheus/client_golang v0.9.3 // indirect
	github.com/spf13/cobra v0.0.4 // indirect
	github.com/spf13/pflag v1.0.3
	github.com/vmware/govmomi v0.20.1
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20190530122614-20be4c3c3ed5 // indirect
	golang.org/x/net v0.0.0-20190603091049-60506f45cf65
	golang.org/x/oauth2 v0.0.0-20190523182746-aaccbc9213b0 // indirect
	golang.org/x/sys v0.0.0-20190422165155-953cdadca894 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190602125759-c1e9adbde704
	k8s.io/apiextensions-apiserver v0.0.0-20190602131520-451a9c13a3c8 // indirect
	k8s.io/apimachinery v0.0.0-20190602125621-c0632ccbde11
	k8s.io/client-go v0.0.0-20190602130007-e65ca70987a6
	k8s.io/cluster-bootstrap v0.0.0-20190531140748-ffdb6fd27ea4
	k8s.io/code-generator v0.0.0-20190531131525-17d711082421
	k8s.io/component-base v0.0.0-20190602130718-4ec519775454 // indirect
	k8s.io/klog v0.3.2
	k8s.io/kube-openapi v0.0.0-20190530181030-b52b5b0f5a7c // indirect
	sigs.k8s.io/cluster-api v0.0.0-20190508175234-0f911c1f65a5
	sigs.k8s.io/controller-runtime v0.1.10
	sigs.k8s.io/controller-tools v0.1.10
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
	sigs.k8s.io/yaml v1.1.0
)
