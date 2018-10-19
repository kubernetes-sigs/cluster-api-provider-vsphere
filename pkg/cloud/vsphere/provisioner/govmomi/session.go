package govmomi

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/soap"
	vsphereutils "sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/utils"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type SessionContext struct {
	session *govmomi.Client
	context *context.Context
	finder  *find.Finder
}

func (pv *Provisioner) sessionFromProviderConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*SessionContext, error) {
	var sc SessionContext
	vsphereConfig, err := vsphereutils.GetClusterProviderSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return nil, err
	}
	if ses, ok := pv.sessioncache[vsphereConfig.VsphereServer+vsphereConfig.VsphereUser]; ok {
		s, ok := ses.(SessionContext)
		if ok {
			// Test if the session is valid and return
			if ok, _ := s.session.SessionManager.SessionIsActive(*s.context); ok {
				return &s, nil
			}
		}
	}
	ctx := context.Background()

	soapURL, err := soap.ParseURL(vsphereConfig.VsphereServer)
	if soapURL == nil || err != nil {
		return nil, fmt.Errorf("error parsing vSphere URL %s : [%s]", soapURL, err)
	}

	// making sure we don't log in during client creation
	soapURL.User = nil

	// Temporarily setting the insecure flag True
	// TODO(ssurana): handle the certs better
	sc.session, err = govmomi.NewClient(ctx, soapURL, true)
	if err != nil {
		return nil, fmt.Errorf("error setting up new vSphere SOAP client: %s", err)
	}

	// TODO(frapposelli): replace `dev` with version string
	sc.session.Client.UserAgent = "kubernetes-cluster-api-provider-vsphere/dev"

	// Set the credentials and login
	// This is done as a separate step to inject the User Agent
	soapURL.User = url.UserPassword(vsphereConfig.VsphereUser, vsphereConfig.VspherePassword)
	if err := sc.session.Login(ctx, soapURL.User); err != nil {
		return nil, fmt.Errorf("error logging into vSphere: %s", err)
	}

	sc.context = &ctx
	finder := find.NewFinder(sc.session.Client, false)
	sc.finder = finder
	pv.sessioncache[vsphereConfig.VsphereServer+vsphereConfig.VsphereUser] = sc
	return &sc, nil
}
