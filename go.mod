module sigs.k8s.io/cluster-api-provider-vsphere

go 1.12

require (
	github.com/go-logr/logr v0.1.0
	github.com/google/uuid v1.1.1
	github.com/mbenkmann/goformat v0.0.0-20180512004123-256ef38c4271
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	github.com/vmware/govmomi v0.20.2
	golang.org/x/lint v0.0.0-20190301231843-5614ed5bae6f
	golang.org/x/tools v0.0.0-20190501045030-23463209683d
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.0.0-20190711103429-37c3b8b1ca65
	k8s.io/apimachinery v0.0.0-20190711103026-7bf792636534
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cluster-bootstrap v0.0.0-20190711112844-b7409fb13d1b
	k8s.io/code-generator v0.0.0-20190311093542-50b561225d70
	k8s.io/gengo v0.0.0-20190813173942-955ffa8fcfc9 // indirect
	k8s.io/klog v0.4.0
	sigs.k8s.io/cluster-api v0.0.0-20190822050630-23ef7a8a48c2
	sigs.k8s.io/controller-runtime v0.2.0-rc.0
	sigs.k8s.io/controller-tools v0.2.0-rc.0
	winterdrache.de/goformat v0.0.0-20180512004123-256ef38c4271 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190704095032-f4ca3d3bdf1d
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190704094733-8f6ac2502e51
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.0.0-20190822050630-23ef7a8a48c2
)
