package context

import (
	"context"
	"net/url"
	"sync"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
)

// NetApp
var restSessionCache = map[string]RestSession{}
var restSessionMU sync.Mutex

// NetApp
type RestSession struct {
	*rest.Client
}

// NetApp
func getOrCreateCachedRESTSession(ctx *MachineContext) (*RestSession, error) {
	restSessionMU.Lock()
	defer restSessionMU.Unlock()

	server := ctx.VSphereCluster.Spec.Server
	datacenter := ctx.VSphereMachine.Spec.Datacenter
	sessionKey := server + ctx.User() + datacenter

	if session, ok := restSessionCache[sessionKey]; ok {
		if ok := session.IsActive(); ok {
			ctx.Logger.V(4).Info("using cached vSphere REST client session", "server", server, "user", ctx.User())
			return &session, nil
		}
	}

	soapURL, err := soap.ParseURL(server)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing vSphere URL %q", server)
	}
	if soapURL == nil {
		return nil, errors.Errorf("error parsing vSphere URL %q", server)
	}

	soapClient := soap.NewClient(soapURL, true)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, errors.Wrapf(err, "error setting up new vSphere SOAP client")
	}

	ctx.Logger.V(2).Info("creating new vSphere REST client session", "server", server, "user", ctx.User())
	restClient := rest.NewClient(vimClient)
	if err := restClient.Login(ctx, url.UserPassword(ctx.User(), ctx.Pass())); err != nil {
		return nil, errors.Wrapf(err, "error logging in with REST client for user %q", ctx.User())
	}

	session := RestSession{Client: restClient}

	session.UserAgent = v1alpha2.GroupVersion.String()

	// Cache the session.
	restSessionCache[sessionKey] = session
	ctx.Logger.V(2).Info("cached vSphere REST client session", "server", server, "user", ctx.User())

	return &session, nil
}

// NetApp
func (s *RestSession) IsActive() bool {

	// NOTE: Rest client does not expose an IsActive check out of the box. Rolling our own rudimentary one.

	tm := tags.NewManager(s.Client)

	_, err := tm.GetTags(context.TODO())
	if err != nil {
		return false
	}

	return true
}