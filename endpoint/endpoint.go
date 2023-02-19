package endpoint

//#cgo LDFLAGS: -lvdeplug -lpthread
//#include <vdeplug.h>
import "C"

import (
	"crypto/rand"
	"errors"
	"net"

	"github.com/docker/go-plugins-helpers/network"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// EndpointStat struct, it is used inside the NetworkStat struct
type EndpointStat struct {

	// used to hold the vde plug PID that connects the endpoint to the vde network
	Plugger uintptr `json:"Plugger"`

	// used as the name of the TAP device associated to the endpoint
	IfName     string `json:"IfName"`
	SandboxKey string `json:"SandboxKey"`

	// IPv4 address of the endpoint
	IPv4Address string `json:"IPv4Address"`

	// IPv6 address for the endpoint
	IPv6Address string `json:"IPv6Address"`

	// MAC address for the TAP decive of the endpoint
	MacAddress string `json:"MacAddress"`
}

// Returns Endpoint Stats for new endpoint
func NewEndpointStat(r *network.CreateEndpointRequest) *EndpointStat {

	// new endpointstat instance
	new := EndpointStat{
		Plugger:     0,
		IfName:      "vde" + r.EndpointID[:11],
		SandboxKey:  "",
		IPv4Address: r.Interface.Address,
		IPv6Address: r.Interface.AddressIPv6,
		MacAddress:  r.Interface.MacAddress,
	}

	// if docker has not provided a MAC Address, assign random one
	if new.MacAddress == "" {
		new.MacAddress = RandomMacAddr()
	}
	return &new
}

func (this *EndpointStat) LinkAdd() error {

	// create attributes struct for new net device with default values
	linkattrs := netlink.NewLinkAttrs()

	// set device name
	linkattrs.Name = this.IfName

	// set MAC address using this endpoint's MAC adress
	linkattrs.HardwareAddr, _ = net.ParseMAC(this.MacAddress)

	// get TUN/TAP device struct with the given attribues
	tapdev := &netlink.Tuntap{LinkAttrs: linkattrs}

	// flag sets that TUN/TAP device should be created without an ip address
	tapdev.Flags = netlink.TUNTAP_NO_PI

	// flag sets that the TUN/TAP device must be TAP device
	tapdev.Mode = netlink.TUNTAP_MODE_TAP

	// adds TAP device, equivalent to "ip tuntap add _ mode tap "
	if err := netlink.LinkAdd(tapdev); err != nil {
		return err
	}

	// sets IPv4 Address to the created TAP device
	if ipv4, err := netlink.ParseAddr(this.IPv4Address); err == nil {
		netlink.AddrAdd(tapdev, ipv4)
	}

	// set IPv6 adress to the created TAP device
	if ipv6, err := netlink.ParseAddr(this.IPv6Address); err == nil {
		netlink.AddrAdd(tapdev, ipv6)
	}
	return nil
}

// Deletes the TAP device associated with the endpoint
func (this *EndpointStat) LinkDel() error {
	var err error
	// retrive the TAP device for this endpoint
	if link, err := netlink.LinkByName(this.IfName); err == nil {
		// delete the tap device
		err = netlink.LinkDel(link)
	}
	return err
}

// Created a VDE plug between endpoint and vdenetwork
func (this *EndpointStat) LinkPlugTo(sock string) error {
	log.Debugf("LinkPlugTo [ %s ] [ %s ]", this.IfName, sock)

	// plugs the newly created TAP device of the endpoint to the given VDE socket, and stores vde plug PID in the endpoint struct
	this.Plugger = uintptr(C.vdeplug_join(C.CString(this.IfName), C.CString(sock)))
	if this.Plugger == 0 {
		return errors.New("LinkPlugTo error: " + this.IfName + " to " + sock)
	}
	return nil
}

// Stops the vde plug that connects the endpoint to the VDE network
func (this *EndpointStat) LinkPlugStop() {
	C.vdeplug_leave(C.uintptr_t(this.Plugger))
	this.Plugger = 0
}

/*
Copied from include/linux/etherdevice.h

	This is the kernel's method of making random mac addresses
*/
func RandomMacAddr() string {
	mac := make([]byte, 6)
	rand.Read(mac)
	mac[0] &= 0xfe /* clear multicast bit */
	mac[0] |= 0x02 /* set local assignment bit (IEEE802) */
	return net.HardwareAddr(mac).String()
}
