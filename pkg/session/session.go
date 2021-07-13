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

package session

import (
	"context"
	"net/url"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

var sessionCache = map[string]Session{}
var sessionMU sync.Mutex

// Session is a vSphere session with a configured Finder.
type Session struct {
	*govmomi.Client
	Finder     *find.Finder
	datacenter *object.Datacenter
	TagManager *tags.Manager
}

// GetOrCreate gets a cached session or creates a new one if one does not
// already exist.
func GetOrCreate(
	ctx context.Context,
	logger logr.Logger,
	server, datacenter, username, password string, thumbprint string) (*Session, error) {

	sessionMU.Lock()
	defer sessionMU.Unlock()

	sessionKey := server + username + datacenter
	if cachedSession, ok := sessionCache[sessionKey]; ok {
		var err error
		if ok, err = cachedSession.SessionManager.SessionIsActive(ctx); ok {
			logger.V(2).Info("found active cached vSphere client session", "server", server, "datacenter", datacenter)
			return &cachedSession, nil
		}
		logger.V(2).Error(err, "error checking if session is active")
	}

	soapURL, err := soap.ParseURL(server)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing vSphere URL %q", server)
	}
	if soapURL == nil {
		return nil, errors.Errorf("error parsing vSphere URL %q", server)
	}

	soapURL.User = url.UserPassword(username, password)
	client, err := newClient(ctx, soapURL, thumbprint)
	if err != nil {
		return nil, err
	}

	session := Session{Client: client}
	session.UserAgent = v1alpha3.GroupVersion.String()

	// Assign the finder to the session.
	session.Finder = find.NewFinder(session.Client.Client, false)
	// Assign tag manager to the session.
	manager, err := newManager(ctx, client.Client, soapURL.User)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create tags manager")
	}
	session.TagManager = manager

	// Assign the datacenter if one was specified.
	dc, err := session.Finder.DatacenterOrDefault(ctx, datacenter)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find datacenter %q", datacenter)
	}
	session.datacenter = dc
	session.Finder.SetDatacenter(dc)

	// Cache the session.
	sessionCache[sessionKey] = session

	logger.V(2).Info("cached vSphere client session", "server", server, "datacenter", datacenter)

	return &session, nil
}

func newClient(ctx context.Context, url *url.URL, thumbprint string) (*govmomi.Client, error) {
	insecure := thumbprint == ""
	soapClient := soap.NewClient(url, insecure)
	if !insecure {
		soapClient.SetThumbprint(url.Host, thumbprint)
	}

	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}
	c := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}
	if err := c.Login(ctx, url.User); err != nil {
		return nil, err
	}

	return c, nil
}

// newManager creates a Manager that encompasses the REST Client for the VSphere tagging API
func newManager(ctx context.Context, client *vim25.Client, user *url.Userinfo) (*tags.Manager, error) {
	rc := rest.NewClient(client)
	err := rc.Login(ctx, user)
	if err != nil {
		return nil, err
	}
	return tags.NewManager(rc), nil
}

// FindByBIOSUUID finds an object by its BIOS UUID.
//
// To avoid comments about this function's name, please see the Golang
// WIKI https://github.com/golang/go/wiki/CodeReviewComments#initialisms.
// This function is named in accordance with the example "XMLHTTP".
func (s *Session) FindByBIOSUUID(ctx context.Context, uuid string) (object.Reference, error) {
	return s.findByUUID(ctx, uuid, false)
}

// FindByInstanceUUID finds an object by its instance UUID.
func (s *Session) FindByInstanceUUID(ctx context.Context, uuid string) (object.Reference, error) {
	return s.findByUUID(ctx, uuid, true)
}

func (s *Session) findByUUID(ctx context.Context, uuid string, findByInstanceUUID bool) (object.Reference, error) {
	if s.Client == nil {
		return nil, errors.New("vSphere client is not initialized")
	}
	si := object.NewSearchIndex(s.Client.Client)
	ref, err := si.FindByUuid(ctx, s.datacenter, uuid, true, &findByInstanceUUID)
	if err != nil {
		return nil, errors.Wrapf(err, "error finding object by uuid %q", uuid)
	}
	return ref, nil
}
