package govmomi

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/soap"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
)

type SessionContext struct {
	session *govmomi.Client
	context *context.Context
	finder  *find.Finder
}

func (pv *Provisioner) sessionFromProviderConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*SessionContext, error) {
	var sc SessionContext
	clusterConfig, err := v1alpha1.ClusterConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}
	username, password, err := pv.GetVsphereCredentials(cluster)
	if err != nil {
		return nil, err
	}
	if ses, ok := pv.sessioncache[clusterConfig.VsphereServer+username]; ok {
		s, ok := ses.(SessionContext)
		if ok {
			// Test if the session is valid and return
			if ok, _ := s.session.SessionManager.SessionIsActive(*s.context); ok {
				return &s, nil
			}
		}
	}
	ctx := context.Background()

	soapURL, err := soap.ParseURL(clusterConfig.VsphereServer)
	if soapURL == nil || err != nil {
		return nil, fmt.Errorf("error parsing vSphere URL %s : [%s]", soapURL, err)
	}

	// Set the credentials
	soapURL.User = url.UserPassword(username, password)

	// Temporarily setting the insecure flag True
	// TODO(ssurana): handle the certs better
	sc.session, err = govmomi.NewClient(ctx, soapURL, true)
	if err != nil {
		return nil, fmt.Errorf("error setting up new vSphere SOAP client: %s", err)
	}

	// TODO(frapposelli): replace `dev` with version string
	sc.session.Client.UserAgent = "kubernetes-cluster-api-provider-vsphere/dev"

	sc.context = &ctx
	finder := find.NewFinder(sc.session.Client, false)
	sc.finder = finder
	pv.sessioncache[clusterConfig.VsphereServer+username] = sc
	return &sc, nil
}
