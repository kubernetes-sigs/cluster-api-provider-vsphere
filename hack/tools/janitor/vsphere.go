/*
Copyright 2024 The Kubernetes Authors.

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

package main

import (
	"context"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	ctrl "sigs.k8s.io/controller-runtime"
)

type getVSphereClientInput struct {
	Password   string
	Server     string
	Thumbprint string
	UserAgent  string
	Username   string
}

// vSphereClients is a collection of different clients for vSphere.
type vSphereClients struct {
	Vim     *vim25.Client
	Govmomi *govmomi.Client
	Rest    *rest.Client
}

// logout logs out all clients. It logs errors if the context contains a logger.
func (v *vSphereClients) logout(ctx context.Context) {
	log := ctrl.LoggerFrom(ctx)
	if err := v.Govmomi.Logout(ctx); err != nil {
		log.Error(err, "logging out govmomi client")
	}

	if err := v.Rest.Logout(ctx); err != nil {
		log.Error(err, "logging out rest client")
	}
}

// newVSphereClients creates a vSphereClients object from the given input.
func newVSphereClients(ctx context.Context, input getVSphereClientInput) (*vSphereClients, error) {
	urlCredentials := url.UserPassword(input.Username, input.Password)

	serverURL, err := soap.ParseURL(input.Server)
	if err != nil {
		return nil, err
	}
	serverURL.User = urlCredentials
	var soapClient *soap.Client
	if input.Thumbprint == "" {
		soapClient = soap.NewClient(serverURL, true)
	} else {
		soapClient = soap.NewClient(serverURL, false)
		soapClient.SetThumbprint(serverURL.Host, input.Thumbprint)
	}
	soapClient.UserAgent = input.UserAgent

	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	govmomiClient := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	if err := govmomiClient.Login(ctx, urlCredentials); err != nil {
		return nil, err
	}

	restClient := rest.NewClient(vimClient)
	if err := restClient.Login(ctx, urlCredentials); err != nil {
		return nil, err
	}

	return &vSphereClients{
		Vim:     vimClient,
		Govmomi: govmomiClient,
		Rest:    restClient,
	}, nil
}
