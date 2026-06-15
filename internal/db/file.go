package filedata

import "log"

// This package will manage staticlly defined users and introduce them to dhcp
// and tftp package
// Milestone:
// 1. Struct based clients
// 2. CSV based clients
// 3. DB based clients -> dbdata
// 4. Pool based broadcast, different subnets for different clusters

type dhcpClients struct {
	userID string
	offerIP string
	macAddress string
	bootFileUrl string
}

func (d *dhcpClients) Populate() {
	log.Println("[Static DB Realm]")
}

type Client interface {
	OfferIP() string
	MACAddress() string
	BootFileUrl() string
}

// Make it into map so it will always be O(1) upon reading from DHCP
type ptrClients struct {
	//Clients []*dhcpClients
	Clients map[string]Client
}

func Gather() *ptrClients {
	log.Println("[Static DB Realm]")
	// Default values
	// Original Client
	newClients1 := dhcpClients{
		offerIP: "192.168.99.200",
		macAddress: "e2:37:36:e8:12:b7",
		bootFileUrl: "pxelinux.0",
	}

	newClients2 := dhcpClients{
		offerIP: "192.168.99.201",
		macAddress: "e2:37:36:e8:63:b5",
		bootFileUrl: "second-user-pxelinux.0",
	}

	clients := []*dhcpClients{
		&newClients1,
		&newClients2,
	}

	m := make(map[string]Client, len(clients))
	for _,c := range clients {
		m[c.macAddress] = c
	}

	return &ptrClients{
		Clients: m,
	}
}

func (d *dhcpClients) OfferIP() string { return d.offerIP }
func (d *dhcpClients) MACAddress() string { return d.macAddress }
func (d *dhcpClients) BootFileUrl() string { return d.bootFileUrl }

