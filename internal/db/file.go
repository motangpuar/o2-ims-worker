package filedata

import "github.com/motangpuar/o2-ims-worker/internal/inventory"
import "log"
import "os"
import "bufio"
import "strings"

//
// This package will manage staticlly defined users and introduce them to dhcp
// and tftp package
// Milestone:
// 1. Struct based clients
// 2. CSV based clients
// 3. DB based clients -> dbdata
// 4. Pool based broadcast, different subnets for different clusters
//

type dhcpClients struct {
	userID string
	offerIP string
	macAddress string
	bootFileUrl string
	osType string
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

func Populate() {
	log.Println("[Static DB Realm]")

	newClients1 := dhcpClients{
		offerIP: "192.168.99.200",
		macAddress: "e2:37:36:e8:12:b7",
		bootFileUrl: "pxelinux.0",
		osType: "centos",
	}

	newClients2 := dhcpClients{
		offerIP: "192.168.99.201",
		macAddress: "e2:37:26:f8:63:b5",
		bootFileUrl: "second-user-pxelinux.0",
		osType: "centos",
	}

	clients := []*dhcpClients{
		&newClients1,
		&newClients2,
	}

	// Process Files
	data, err := os.Open("inputs/clients.csv")
	if err != nil {
		log.Printf("Failed to open file %v", err)
	}

	// Run last 
	defer data.Close()

	scanner := bufio.NewScanner(data)
	if scanner.Err() != nil {
		log.Fatalf("Failed to scan CSV file: %s", err)
	}


	// Skip first line by start an empty Scan() function from scanner
	if scanner.Scan() {}

	for scanner.Scan() {
		cLine := scanner.Text()
		//log.Printf("[BUFIO] %s", cLine)
		read_lines := strings.Split(cLine, ",")
		clients = append(clients,
			&dhcpClients{
				offerIP: read_lines[0],
				macAddress: read_lines[1],
				bootFileUrl: read_lines[2],
				osType: read_lines[3],
			},
		)
	}

	m := make(map[string]Client, len(clients))
	for _,c := range clients {
		log.Printf("[Struct] %s", c)
		m[c.macAddress] = c
		inventory.Generate(c.macAddress, c.osType)
	}
	log.Printf("[Struct] Total Section %d", len(clients))

	// Intialize pointer of Clients
	_ = &ptrClients{
		Clients: m,
	}
}

// Return Pointer when asked
func Gather() *ptrClients {
	return &ptrClients{}
}


// Client Specific Values
func (d *dhcpClients) OfferIP() string { return d.offerIP }
func (d *dhcpClients) MACAddress() string { return d.macAddress }
func (d *dhcpClients) BootFileUrl() string { return d.bootFileUrl }

