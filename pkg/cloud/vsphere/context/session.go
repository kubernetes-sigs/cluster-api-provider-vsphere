/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package context

import (
	"context"
	"net/url"
	"sync"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
)

var sessionCache = map[string]Session{}
var sessionMU sync.Mutex

// Session is a vSphere session with a configured Finder.
type Session struct {
	*govmomi.Client
	Finder     *find.Finder
	datacenter *object.Datacenter
}

func getOrCreateCachedSession(ctx *MachineContext) (*Session, error) {
	sessionMU.Lock()
	defer sessionMU.Unlock()

	datacenter := ctx.MachineConfig.MachineSpec.Datacenter
	sessionKey := ctx.ClusterConfig.VsphereServer + ctx.User() + datacenter

	if session, ok := sessionCache[sessionKey]; ok {
		if ok, _ := session.SessionManager.SessionIsActive(ctx); ok {
			return &session, nil
		}
	}

	soapURL, err := soap.ParseURL(ctx.ClusterConfig.VsphereServer)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing vSphere URL %q", ctx.ClusterConfig.VsphereServer)
	}
	if soapURL == nil {
		return nil, errors.Errorf("error parsing vSphere URL %q", ctx.ClusterConfig.VsphereServer)
	}

	soapURL.User = url.UserPassword(ctx.User(), ctx.Pass())

	// Temporarily setting the insecure flag True
	// TODO(ssurana): handle the certs better
	client, err := govmomi.NewClient(ctx, soapURL, true)
	if err != nil {
		return nil, errors.Wrapf(err, "error setting up new vSphere SOAP client")
	}

	session := Session{Client: client}

	// TODO(frapposelli): replace `dev` with version string
	session.UserAgent = "kubernetes-cluster-api-provider-vsphere/dev"

	// Assign the finder to the session.
	session.Finder = find.NewFinder(session.Client.Client, false)

	// Assign the datacenter if one was specified.
	dc, err := session.Finder.DatacenterOrDefault(ctx, datacenter)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find datacenter %q", datacenter)
	}
	session.datacenter = dc
	session.Finder.SetDatacenter(dc)

	// Cache the session.
	sessionCache[sessionKey] = session
	ctx.Logger.V(2).Info("cached vSphere client session", "server", ctx.ClusterConfig.VsphereServer, "datacenter", datacenter)

	return &session, nil
}

// FindByInstanceUUID finds an object by its instance UUID.
func (s *Session) FindByInstanceUUID(ctx context.Context, uuid string) (object.Reference, error) {
	if s.Client == nil {
		return nil, errors.New("vSphere client is not initialized")
	}
	si := object.NewSearchIndex(s.Client.Client)
	findFlag := true
	ref, err := si.FindByUuid(ctx, s.datacenter, uuid, true, &findFlag)
	if err != nil {
		return nil, errors.Wrapf(err, "error finding object by instance uuid %q", uuid)
	}
	return ref, nil
}

// FindByUUID finds an object by its UUID.
func (s *Session) FindByUUID(ctx context.Context, uuid string) (object.Reference, error) {
	if s.Client == nil {
		return nil, errors.New("vSphere client is not initialized")
	}
	si := object.NewSearchIndex(s.Client.Client)
	findFlag := false
	ref, err := si.FindByUuid(ctx, s.datacenter, uuid, true, &findFlag)
	if err != nil {
		return nil, errors.Wrapf(err, "error finding object by uuid %q", uuid)
	}
	return ref, nil
}
