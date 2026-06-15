package main

import "log"
import "flag"
import "net"
import "time"
import "github.com/insomniacslk/dhcp/dhcpv4"
import "github.com/insomniacslk/dhcp/dhcpv4/client4"


func main() {

	ifaceName := flag.String("i","dummy-client","Interface for DHCP Client")
	flag.Parse()
	log.Printf("Staring DHCP Client at %s", *ifaceName)

	_,err := net.InterfaceByName(*ifaceName)
	if err != nil {
		log.Fatalf("Interface Error: %v", err)
	}

	// discover, err := dhcpv4.NewDiscovery(iface.HardwareAddr)
	// if err != nil {
	// 	log.Fatalf("Payload Construcition error: %v", err)
	// } else {
	// 	log.Printf("Discover at MAC: %s", discover)
	// }

	client := client4.NewClient()
	modifiers := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover), // Redundant, but explicit
		dhcpv4.WithRequestedOptions(dhcpv4.OptionRouter, dhcpv4.OptionDomainNameServer),
		dhcpv4.WithBroadcast(true),
	}
	client.ReadTimeout = 5 * time.Second
	conversation, err := client.Exchange(*ifaceName, modifiers...)
	if err != nil {
		log.Fatalf("Exchange error: %v", err)
	}

	for _,packet := range conversation {
		log.Print(packet.Summary())
	}

	if err != nil {
		log.Fatal(err)
	}
}


