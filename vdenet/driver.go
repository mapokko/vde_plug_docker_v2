// Impementation of the network driver interface
package vdenet

import (
	"net"
	"strings"
	"sync"

	"phocs/vde_plug_docker/datastore"
	"phocs/vde_plug_docker/endpoint"

	"github.com/docker/go-plugins-helpers/network"
	"github.com/docker/libnetwork/types"
	log "github.com/sirupsen/logrus"
)

// Holds the info about specific vde network
type NetworkStat struct {
	// VDE network socket in VNL syntax (e.g. vxvde://239.1.2.3)
	Sock string `json:"Sock"`

	// used as the prefix to name the TAP devices associated with the endpoint of this network
	IfPrefix string `json:"IfPrefix"`

	// range of IP Addresses represented in CIDR format address/mask
	IPv4Pool string `json:"IPv4Pool"`

	// optional, gateway IP address in CIDR format for the subnet represented by the Pool
	IPv4Gateway string `json:"IPv4Gateway"`
	IPv6Pool    string `json:"IPv6Pool"`
	IPv6Gateway string `json:"IPv6Gateway"`

	// key-value pairs where keys are the endpointID and the values are the endpoint struct
	Endpoints map[string]*endpoint.EndpointStat `json:"Endpoints"`
}

// driver struct, holds the info about networks,a mutex to edit them concurrently and all the required methods by the Docker network extension API, it is also stores ad a JSON file
type Driver struct {

	//mutex to edit the networks and avoid race condition
	mutex sync.RWMutex `json:"-"` // ignore

	// key-value pairs where values are the network struct and the keys are the network ID provided by docker when calling CreateNetwork()
	Networks map[string]*NetworkStat `json:"Networks"`
}

// default prefix used to name the endpoint's interface name
const (
	IfPrefixDefault = "vde"
)

// creates and returns a network driver following the Docker network extension API https://github.com/docker/go-plugins-helpers/blob/master/network/api.go
func NewDriver(storepath string, clean bool) Driver {
	// instantiate new driver with empty networks
	driver := Driver{Networks: make(map[string]*NetworkStat)}

	// set datastore path
	datastore.SetPath(storepath)

	// if clean flag is true, empties datastore file
	if clean == true {
		datastore.Clean()

		// else, loads previous datastore networks in driver
	} else if err := datastore.Load(&driver); err == nil {
		// Check the old Driver data, nwkey id the networkID, nw is the NetworkStat instance
		for nwkey, nw := range driver.Networks {
			//Check each endpoint of every network, epkey is the EndpointID, ep is EndpointStat instance
			for epkey, ep := range nw.Endpoints {
				if ep.Plugger == 0 || ep.LinkDel() == nil {
					/* Container has been stopped or is running (whitout plugger) */
					delete(driver.Networks[nwkey].Endpoints, epkey)
				}
			}
		}
		// stores the driver networks in the datastore
		_ = datastore.Store(&driver)
	}
	return driver
}

/* CapabilitiesResponse returns whether or not this network is global or local, */
func (this *Driver) GetCapabilities() (*network.CapabilitiesResponse, error) {
	return &network.CapabilitiesResponse{Scope: network.LocalScope}, nil
}

// Driver method that creates a new network, receives a CreateNetworkRequest as parameter when a network needs to be created
func (this *Driver) CreateNetwork(r *network.CreateNetworkRequest) error {
	log.Debugf("Createnetwork Request: [ %+v ]", r)

	var sock, ifprefix, ipv6pool, ipv6gateway string

	// opt contains the options passed when creating the docker vde network
	opt := r.Options["com.docker.network.generic"].(map[string]interface{})

	// error if there are no IPv4 address information
	if r.IPv4Data == nil || len(r.IPv4Data) == 0 {
		return types.BadRequestErrorf("Network IPv4Data config miss.")
	}

	// error if socket is missing in the options
	if sock, _ = opt["sock"].(string); sock == "" {
		return types.NotFoundErrorf("Sock URL miss.")
	}

	// if interface prefix is missing, use default interface prefix
	if ifprefix, _ = opt["if"].(string); ifprefix == "" {
		ifprefix = IfPrefixDefault
	}

	// error if interface name prefix exceeds 4 characters
	if len(ifprefix) > 4 {
		return types.BadRequestErrorf("Interface prefix exceeds 4 character limit.")
	}

	// if there is any IPv6 information, set it
	if r.IPv6Data != nil && len(r.IPv6Data) > 0 {
		ipv6pool = r.IPv6Data[0].Pool
		ipv6gateway = r.IPv6Data[0].Gateway
	}

	// lock driver mutex
	this.mutex.Lock()

	// unlock driver lock function ends
	defer this.mutex.Unlock()

	// store driver networks when function ends
	defer datastore.Store(&this)

	// add network to driver, r.NetworkID has
	this.Networks[r.NetworkID] = &NetworkStat{
		Sock:        sock,
		IfPrefix:    ifprefix,
		IPv4Pool:    r.IPv4Data[0].Pool,
		IPv4Gateway: r.IPv4Data[0].Gateway,
		IPv6Pool:    ipv6pool,
		IPv6Gateway: ipv6gateway,

		// empty endpoint struct
		Endpoints: make(map[string]*endpoint.EndpointStat),
	}
	return nil
}

// Not implemented
func (this *Driver) AllocateNetwork(r *network.AllocateNetworkRequest) (*network.AllocateNetworkResponse, error) {
	//  log.Debugf("Allocatenetwork Request: [ %+v ]", r)
	return nil, types.NotImplementedErrorf("Not implementethis.")
}

// Called when a network needs to be removec, deletes a network
func (this *Driver) DeleteNetwork(r *network.DeleteNetworkRequest) error {
	log.Debugf("Deletenetwork: [ %+v ]", r)
	var netw *NetworkStat

	// lock the driver struct
	this.mutex.Lock()

	// unlock the driver struct when function ends
	defer this.mutex.Unlock()

	// error if the network ID provided by docker do not exist
	if netw = this.Networks[r.NetworkID]; netw == nil {
		return types.NotFoundErrorf("Network not found.")
	}

	// error if there are active continers connected to the network
	if len(netw.Endpoints) != 0 {
		return types.BadRequestErrorf("There are still active endpoints.")
	}

	// delete specific network from driver struct
	delete(this.Networks, r.NetworkID)

	// store the now updated driver onto the datastorage
	_ = datastore.Store(&this)
	return nil
}

// Not implemented
func (this *Driver) FreeNetwork(r *network.FreeNetworkRequest) error {
	//  log.Warnf("Freenetwork Request: [ %+v ]", r)
	return types.NotImplementedErrorf("Not implementethis.")
}

// Called when an endpoint should be created, creates an endpoint for the container
func (this *Driver) CreateEndpoint(r *network.CreateEndpointRequest) (*network.CreateEndpointResponse, error) {
	log.Debugf("CREATE ENDPOINT: [ %+v ]", r)

	// lock driver struct
	this.mutex.Lock()

	//unlock driver struct when function ends
	defer this.mutex.Unlock()

	// get the struct of the network to which the endpoint must be connected
	netw := this.Networks[r.NetworkID]

	// error if the network is not found
	if netw == nil {
		return nil, types.NotFoundErrorf("Network not found.")
	}

	// error if endpoint with provided endpoitnID already exists
	if netw.Endpoints[r.EndpointID] != nil {
		return nil, types.BadRequestErrorf("EndpointID already exists.")
	}

	// populates new endpoint with initial stats
	netw.Endpoints[r.EndpointID] = endpoint.NewEndpointStat(r, netw.IfPrefix)

	// create reponse using CreateEndpointResponse provided by go-plugins-helpers
	response := &network.CreateEndpointResponse{
		// assings and empty EndpointInterface to the Interface attribute
		// it must be empty when r.Iterface is non-nil
		Interface: &network.EndpointInterface{},
	}

	// if the request doesnt provide a macAdress, assign the generated one
	if r.Interface.MacAddress == "" {
		response.Interface.MacAddress = netw.Endpoints[r.EndpointID].MacAddress
	}

	// save the driver data in the datastore
	_ = datastore.Store(&this)

	//send created response
	return response, nil
}

// Deletes the endpoint which's id is provided as argument
func (this *Driver) DeleteEndpoint(r *network.DeleteEndpointRequest) error {
	log.Debugf("DeleteEndpoint: [ %+v ]", r)

	// lock driver mutex
	this.mutex.Lock()

	//unlock driver mutex when functin ends
	defer this.mutex.Unlock()

	// error if the network doesn't exists
	if this.Networks[r.NetworkID] == nil {
		return types.NotFoundErrorf("Network not found.")
	}

	// error if the requested endpoint to delete doesnt exist
	if this.Networks[r.NetworkID].Endpoints[r.EndpointID] == nil {
		return types.NotFoundErrorf("Endpoint not found.")
	}

	// deletes link between endpoint and VDE network
	this.Networks[r.NetworkID].Endpoints[r.EndpointID].LinkDel()

	// deletes endppoint data from driver
	delete(this.Networks[r.NetworkID].Endpoints, r.EndpointID)

	// saves driver in datastore
	_ = datastore.Store(&this)
	return nil
}

// Called when docker requests info about endpoint
func (this *Driver) EndpointInfo(r *network.InfoRequest) (*network.InfoResponse, error) {
	log.Debugf("ENDPOINT INFO: [ %+v ]", r)

	// locks Driver read
	this.mutex.RLock()

	// unlock driver read when functions ends
	defer this.mutex.RUnlock()

	// error if network not found
	if this.Networks[r.NetworkID] == nil {
		return nil, types.NotFoundErrorf("network not found.")
	}

	// error if endpoint is not found
	if this.Networks[r.NetworkID].Endpoints[r.EndpointID] == nil {
		return nil, types.NotFoundErrorf("Endpoint not found.")
	}

	// create empty InfoResponse struct
	info := &network.InfoResponse{Value: make(map[string]string)}

	// set reponse id with provided ID
	info.Value["id"] = r.EndpointID

	// set the TAP interface name in the docker network namespace
	info.Value["srcName"] = this.Networks[r.NetworkID].Endpoints[r.EndpointID].IfName

	log.Debugf("In EndpointInfo: ", this.Networks[r.NetworkID].Endpoints[r.EndpointID].IfName)

	return info, nil
}

// Called when and endpoint must be joined to a network
func (this *Driver) Join(r *network.JoinRequest) (*network.JoinResponse, error) {
	log.Debugf("JOIN: [ %+v ]", r)

	var netw *NetworkStat
	var edpt *endpoint.EndpointStat
	var gateway, gateway6 string

	// lock driver mutex
	this.mutex.Lock()

	// unlock driver mutex when function ends
	defer this.mutex.Unlock()

	// error if the network isnt found
	if netw = this.Networks[r.NetworkID]; netw == nil {
		return nil, types.NotFoundErrorf("Network not found.")
	}

	// error if the endpoint isnt found
	if edpt = netw.Endpoints[r.EndpointID]; edpt == nil {
		return nil, types.NotFoundErrorf("Endpoint not found.")
	}

	// sets up a tap device with the endpoint's IP addresses
	if edpt.LinkAdd() != nil {
		return nil, types.RetryErrorf("Failed link create.")
	}

	// use a VDE plug to plug the endpoint to the VDE network with the given VNL
	if err := edpt.LinkPlugTo(netw.Sock); err != nil {
		edpt.LinkDel()
		return nil, types.NotFoundErrorf("Failed plug to interface", err)
	}

	// add SandboxKey to Endpoint struct
	edpt.SandboxKey = r.SandboxKey

	// remove subnet mask from IPv4 gateway
	if netw.IPv4Gateway != "" {
		gateway = net.ParseIP(strings.Split(netw.IPv4Gateway, "/")[0]).String()
	}

	// remove subnet mask from IPv6 gateway
	if netw.IPv6Gateway != "" {
		gateway6 = net.ParseIP(strings.Split(netw.IPv6Gateway, "/")[0]).String()
	}

	// create a response for the join operation, following the pattern sepcified in the documentation
	response := &network.JoinResponse{

		// information about the host level TAP interface created for this endpoint, LibNetwork moves it inside the sandbox
		InterfaceName: network.InterfaceName{
			// name of the TAP interface
			SrcName: edpt.IfName,
			// prefix for the TAP interface name inside the sandbox
			DstPrefix: netw.IfPrefix,
		},
		Gateway:     gateway,
		GatewayIPv6: gateway6,
	}
	_ = datastore.Store(&this)
	return response, nil
}

// Called when an endpoint is leaving the network
func (this *Driver) Leave(r *network.LeaveRequest) error {
	log.Debugf("LEAVE: [ %+v ]", r)
	var netw *NetworkStat
	var edpt *endpoint.EndpointStat

	// lock the driver
	this.mutex.Lock()

	//unlock the driver when function ends
	defer this.mutex.Unlock()

	// error if the network isnt found
	if netw = this.Networks[r.NetworkID]; netw == nil {
		return types.NotFoundErrorf("network not found.")
	}

	// error if the endpoint isnt found
	if edpt = netw.Endpoints[r.EndpointID]; edpt == nil {
		return types.NotFoundErrorf("Endpoint not found.")
	}

	// stops the vde plug connecting the endpoint to the vde network
	edpt.LinkPlugStop()

	// deletes the TAP device for this endpoint
	edpt.LinkDel()

	// updates datastore
	_ = datastore.Store(&this)
	return nil
}

// Not implemented
func (this *Driver) DiscoverNew(r *network.DiscoveryNotification) error {
	//  log.Debugf("DISCOVER NEW Called: [ %+v ]", r)
	return nil
}

// Not implemented

func (this *Driver) DiscoverDelete(r *network.DiscoveryNotification) error {
	//  log.Debugf("DISCOVER DELETE Called: [ %+v ]", r)
	return nil
}

// Not implemented

func (this *Driver) ProgramExternalConnectivity(r *network.ProgramExternalConnectivityRequest) error {
	//  log.Debugf("PROGRAM EXTERNAL CONNECTIVITY Called: [ %+v ]", r)
	return nil
}

// Not implemented

func (this *Driver) RevokeExternalConnectivity(r *network.RevokeExternalConnectivityRequest) error {
	//  log.Debugf("REVOKE EXTERNAL CONNECTIVITY Called: [ %+v ]", r)
	return nil
}
