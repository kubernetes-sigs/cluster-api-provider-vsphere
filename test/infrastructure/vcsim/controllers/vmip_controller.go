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

package controllers

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi/vim25/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	govmominet "sigs.k8s.io/cluster-api-provider-vsphere/pkg/services/govmomi/net"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/session"
)

type vmIPReconciler struct {
	Client            client.Client
	EnableKeepAlive   bool
	KeepAliveDuration time.Duration

	IsVMWaitingforIP  func() bool
	GetVCenterSession func(ctx context.Context) (*session.Session, error)
	GetVMPath         func() string
}

func (r *vmIPReconciler) ReconcileIP(ctx context.Context) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// No op if f the VM is still provisioning, or it already has an IP, return.
	if !r.IsVMWaitingforIP() {
		return reconcile.Result{}, nil
	}

	// Otherwise the VM is stuck waiting for IP (because there is no DHCP service in vcsim), then assign a fake IP.

	authSession, err := r.GetVCenterSession(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get vcenter session")
	}

	vm, err := authSession.Finder.VirtualMachine(ctx, r.GetVMPath())
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to find vm")
	}

	// Check if the VM already has network status (but it is not yet surfaced in conditions)
	netStatus, err := govmominet.GetNetworkStatus(ctx, authSession.Client.Client, vm.Reference())
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get vm network status")
	}
	ipAddrs := []string{}
	for _, s := range netStatus {
		ipAddrs = append(ipAddrs, s.IPAddrs...)
	}
	if len(ipAddrs) > 0 {
		// No op, the VM already has an IP, we should just wait for it to surface in K8s VirtualMachine/VSphereVM
		return reconcile.Result{}, nil
	}

	log.Info("Powering Off the VM before applying an IP")
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to PowerOff vm")
	}
	if err = task.Wait(ctx); err != nil { // deprecation on Wait is going to be removed, see https://github.com/vmware/govmomi/issues/3394
		return reconcile.Result{}, errors.Wrapf(err, "failed to PowerOff vm task to complete")
	}

	// Add a fake ip address.
	spec := types.CustomizationSpec{
		NicSettingMap: []types.CustomizationAdapterMapping{
			{
				Adapter: types.CustomizationIPSettings{
					Ip: &types.CustomizationFixedIp{
						IpAddress: "192.168.1.100",
					},
					SubnetMask:    "255.255.255.0",
					Gateway:       []string{"192.168.1.1"},
					DnsServerList: []string{"192.168.1.1"},
					DnsDomain:     "ad.domain",
				},
			},
		},
		Identity: &types.CustomizationLinuxPrep{
			HostName: &types.CustomizationFixedName{
				Name: "hostname",
			},
			Domain:     "ad.domain",
			TimeZone:   "Etc/UTC",
			HwClockUTC: types.NewBool(true),
		},
		GlobalIPSettings: types.CustomizationGlobalIPSettings{
			DnsSuffixList: []string{"ad.domain"},
			DnsServerList: []string{"192.168.1.1"},
		},
	}

	log.Info("Customizing the VM for applying an IP")
	task, err = vm.Customize(ctx, spec)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to Customize vm")
	}
	if err = task.Wait(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to wait for Customize vm task to complete")
	}

	log.Info("Powering On the VM after applying the IP")
	task, err = vm.PowerOn(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to PowerOn vm")
	}
	if err = task.Wait(ctx); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to PowerOn vm task to complete")
	}

	ip, err := vm.WaitForIP(ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to WaitForIP")
	}
	log.Info("IP assigned to the VM", "ip", ip)

	return reconcile.Result{}, nil
}
