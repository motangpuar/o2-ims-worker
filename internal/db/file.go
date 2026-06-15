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

type ptrClients struct {
	//Clients []*dhcpClients
	Clients []Client
}

func Gather() *ptrClients {
	log.Println("[Static DB Realm]")
	// Default values
	// Original Client
	newClients1 := dhcpClients{
		offerIP: "192.168.99.200",
		macAddress: "e2:37:36:e8:63:b5",
		bootFileUrl: "pxelinux.0",
	}

	newClients2 := dhcpClients{
		offerIP: "192.168.99.201",
		macAddress: "e2:37:36:e8:63:b5",
		bootFileUrl: "pxelinux.0",
	}

	return &ptrClients{
		Clients: []Client{
			&newClients1,
			&newClients2,
		},
	}
}

func (d *dhcpClients) OfferIP() string { return d.offerIP }
func (d *dhcpClients) MACAddress() string { return d.macAddress }
func (d *dhcpClients) BootFileUrl() string { return d.bootFileUrl }

