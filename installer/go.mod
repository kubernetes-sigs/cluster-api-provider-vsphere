module sigs.k8s.io/cluster-api-provider-vsphere-installer

go 1.12

replace sigs.k8s.io/cluster-api-provider-vsphere => ../

require (
	github.com/GoASTScanner/gas v0.0.0-20171004140147-6de76c92610b
	github.com/coreos/go-systemd v0.0.0-20181031085051-9002847aa142
	github.com/davecgh/go-xdr v0.0.0-20170217172119-4930550ba2e2 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/gizak/termui v2.3.0+incompatible
	github.com/maruel/panicparse v0.0.0-20180806203336-f20d4c4d746f // indirect
	github.com/mattn/go-runewidth v0.0.3 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/nbutton23/zxcvbn-go v0.0.0-20180912185939-ae427f1e4c1d // indirect
	github.com/nsf/termbox-go v0.0.0-20181027232701-60ab7e3d12ed // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sirupsen/logrus v1.2.0
	github.com/spf13/pflag v1.0.3
	github.com/vmware/govmomi v0.20.1
	github.com/vmware/vmw-guestinfo v0.0.0-20170707015358-25eff159a728
	golang.org/x/crypto v0.0.0-20190530122614-20be4c3c3ed5
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/sys v0.0.0-20190422165155-953cdadca894 // indirect
	gopkg.in/urfave/cli.v1 v1.20.0
)
