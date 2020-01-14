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

package nsxt

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	nsxt "github.com/vmware/go-vmware-nsxt"
	"github.com/vmware/go-vmware-nsxt/loadbalancer"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

const (
	defaultAPIEndpointPort = int32(6443)
)

// NsxtLB is structure that enable nsxt client and config access
type NsxtLB struct {
	Client *nsxt.APIClient
	Config *nsxt.Configuration
}

// New creates an NsxtLB struct
func New(client *nsxt.APIClient, config *nsxt.Configuration) *NsxtLB {
	return &NsxtLB{
		Client: client,
		Config: config,
	}
}

// GetLoadBalancerName gets the nsx-t load balancer name from the loadBalancer type
func (n *NsxtLB) GetLoadBalancerName(loadBalancer *infrav1.NSXTLoadBalancer) string {
	// NSX-T LB name is in the format <service-namespace>-<service-name>-<first-five-chars-service-uuid>.
	// The UUID in the end is the ensure LB names are unique across clusters
	return fmt.Sprintf("%s-%s-%s", loadBalancer.Namespace, loadBalancer.Name, loadBalancer.UID[:5])
}

// GetVirtualServerName gets the virtual server name
func (n *NsxtLB) GetVirtualServerName(lbName string, port int32) string {
	return fmt.Sprintf("%s-port-%d", lbName, port)
}

// AddVirtualServersToLoadBalancer adds virtual servers to a Load balancer
func (n *NsxtLB) AddVirtualServersToLoadBalancer(virtualServerIDs []string, lbServiceID string) error {
	if len(virtualServerIDs) == 0 {
		return nil
	}

	// first read load balancer service
	lbService, _, err := n.Client.ServicesApi.ReadLoadBalancerService(n.Client.Context, lbServiceID)
	if err != nil {
		return err
	}

	newVirtualServerIDs := append(lbService.VirtualServerIds, virtualServerIDs...)
	lbService.VirtualServerIds = newVirtualServerIDs

	_, _, err = n.Client.ServicesApi.UpdateLoadBalancerService(n.Client.Context, lbService.Id, lbService)
	return err
}

// GetVirtualServers returns a list of virtualServers for this loadBalancer
func (n *NsxtLB) GetVirtualServers(loadBalancer *infrav1.NSXTLoadBalancer) ([]loadbalancer.LbVirtualServer, error) {
	lbName := n.GetLoadBalancerName(loadBalancer)

	allVirtualServers, err := n.listLoadBalancerVirtualServers()
	if err != nil {
		return nil, err
	}

	virtualServerNames := sets.NewString()
	port := defaultAPIEndpointPort
	virtualServerNames.Insert(n.GetVirtualServerName(lbName, port))

	virtualServers := []loadbalancer.LbVirtualServer{}
	for _, virtualServer := range allVirtualServers {
		if !virtualServerNames.Has(virtualServer.DisplayName) {
			continue
		}

		virtualServers = append(virtualServers, virtualServer)
	}

	return virtualServers, nil
}

func (n *NsxtLB) GetUniqueIPsFromVirtualServers(lbs []loadbalancer.LbVirtualServer) []string {
	ipSet := sets.NewString()
	for _, lb := range lbs {
		if ipSet.Has(lb.IpAddress) {
			continue
		}

		ipSet.Insert(lb.IpAddress)
	}

	return ipSet.List()
}

func (n *NsxtLB) getLBServiceByName(name string) (loadbalancer.LbService, bool, error) {
	lbs, err := n.listLoadBalancerServices()
	if err != nil {
		return loadbalancer.LbService{}, false, err
	}

	for _, lbSvc := range lbs {
		if lbSvc.DisplayName != name {
			continue
		}

		return lbSvc, true, nil
	}

	return loadbalancer.LbService{}, false, nil
}

// GetVirtualServerByName gets virtual server that is named name
func (n *NsxtLB) GetVirtualServerByName(name string) (loadbalancer.LbVirtualServer, bool, error) {
	virtualServers, err := n.listLoadBalancerVirtualServers()
	if err != nil {
		return loadbalancer.LbVirtualServer{}, false, err
	}

	for _, virtualServer := range virtualServers {
		if virtualServer.DisplayName != name {
			continue
		}

		return virtualServer, true, nil
	}

	return loadbalancer.LbVirtualServer{}, false, nil
}

// CreateOrUpdateLBPool create Loadbalancer pool if it doesn't exist or update the existing one
func (n *NsxtLB) CreateOrUpdateLBPool(lbName string, lbMembers []loadbalancer.PoolMember) (loadbalancer.LbPool, error) {
	lbPool, exists, err := n.GetLBPoolByName(lbName)
	if err != nil {
		return loadbalancer.LbPool{}, err
	}

	lbPoolID := lbPool.Id
	lbPool = loadbalancer.LbPool{
		//  TODO: LB pool algorithm should be configurable via an annotation on the Service
		Algorithm:        "ROUND_ROBIN",
		DisplayName:      lbName,
		Description:      fmt.Sprintf("LoadBalancer Pool managed by CAPV"),
		Members:          lbMembers,
		MinActiveMembers: 1,
	}

	if !exists {
		lbPool, _, err = n.Client.ServicesApi.CreateLoadBalancerPool(n.Client.Context, lbPool)
		if err != nil {
			return loadbalancer.LbPool{}, err
		}
	} else {
		lbPool, _, err = n.Client.ServicesApi.UpdateLoadBalancerPool(n.Client.Context, lbPoolID, lbPool)
		if err != nil {
			return loadbalancer.LbPool{}, err
		}
	}

	return lbPool, nil
}

// GetLBPoolByName gets loadBlancer pool named name
func (n *NsxtLB) GetLBPoolByName(name string) (loadbalancer.LbPool, bool, error) {
	lbPools, err := n.listLoadBalancerPool()
	if err != nil {
		return loadbalancer.LbPool{}, false, err
	}

	for _, lbPool := range lbPools {
		if lbPool.DisplayName != name {
			continue
		}

		return lbPool, true, nil
	}

	return loadbalancer.LbPool{}, false, nil
}

// MachinesToLBMembers transforms machines into load balancer pool members
func (n *NsxtLB) MachinesToLBMembers(machines []*clusterv1.Machine) []loadbalancer.PoolMember {
	var lbMembers []loadbalancer.PoolMember
	for _, machine := range machines {
		// TODO: don't always assume InternalIP from node addresses
		addresses := machine.Status.Addresses
		for _, address := range addresses {
			if address.Type != clusterv1.MachineExternalIP {
				continue
			}

			member := loadbalancer.PoolMember{
				DisplayName: machine.Name,
				Weight:      1,
				IpAddress:   address.Address,
			}

			lbMembers = append(lbMembers, member)

			// only the first reported External IP to pool members
			break
		}
	}

	return lbMembers
}

func getInternalIP(node *v1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type != v1.NodeInternalIP {
			continue
		}

		return address.Address
	}

	return ""
}

// ListLoadBalancerVirtualServers represents the http response from list load balancer virtual server request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerVirtualServers struct {
	ResultCount int                            `json:"result_count"`
	Results     []loadbalancer.LbVirtualServer `json:"results"`
}

// listLoadBalancerVirtualServers makes an http request for listing load balancer virtual servers
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *NsxtLB) listLoadBalancerVirtualServers() ([]loadbalancer.LbVirtualServer, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.Config.Host + "/api/v1/loadbalancer/virtual-servers"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(n.Config.UserName, n.Config.Password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var results ListLoadBalancerVirtualServers
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}

// ListLoadBalancerService represents the http response from list load balancer service request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerService struct {
	ResultCount int                      `json:"result_count"`
	Results     []loadbalancer.LbService `json:"results"`
}

// listLoadBalancers makes an http request for listing load balancer services
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *NsxtLB) listLoadBalancerServices() ([]loadbalancer.LbService, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.Config.Host + "/api/v1/loadbalancer/services"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(n.Config.UserName, n.Config.Password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var results ListLoadBalancerService
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}

// ListLoadBalancerPool represents the http response from list load balancer pools request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerPool struct {
	ResultCount int                   `json:"result_count"`
	Results     []loadbalancer.LbPool `json:"results"`
}

// listLoadBalancerPool makes an http request for listing load balancer pools
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *NsxtLB) listLoadBalancerPool() ([]loadbalancer.LbPool, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.Config.Host + "/api/v1/loadbalancer/pools"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(n.Config.UserName, n.Config.Password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var results ListLoadBalancerPool
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}
