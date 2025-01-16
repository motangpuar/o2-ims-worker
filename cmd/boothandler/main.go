package main

import (
        //DHCPv4 insomnicaslk
        "time"
        "fmt"
        "net"
        "net/netip"
        "log"
        "github.com/insomniacslk/dhcp/dhcpv4"
        "github.com/insomniacslk/dhcp/dhcpv4/server4"

        // TFTP
        "io"
        "os"
        "github.com/pin/tftp/v3"
        "context"

        // Internals
        "git.nnag.me/infidel/boothandler-go/internal/db"
        "git.nnag.me/infidel/boothandler-go/internal/api"
)

type Packet struct {
    Peer net.Addr
    Pkt *dhcpv4.DHCPv4
    Md *Metadata
}

type Metadata struct {
    IfName string
    IfIndex int
}

type dhcpConfig struct {
    enabled bool
    mode    string
    bindAddr    string
    bindInterface string
    tftpIP  string
    tftpPort    int
}

type tftpConfig struct {
    enabled bool
    bindAddr string
    bindPort int
    blockSize int
}

func main() {

    // Get worker ID (could be hostname or container ID)
    workerID, err := os.Hostname()
    if err != nil {
        log.Fatalf("Failed to get hostname: %v", err)
    }

    // Get credentials from environment variables
    username := os.Getenv("IMS_API_USERNAME")
    password := os.Getenv("IMS_API_PASSWORD")
    apiURL := os.Getenv("IMS_API_URL")

    if username == "" || password == "" || apiURL == "" {
        log.Fatal("IMS_API_USERNAME, IMS_API_PASSWORD and IMS_API_URL environment variables are required")
    }

    // Create API client with authentication
    apiClient := api.NewAPIClient(apiURL, workerID, username, password)

    // Get IP address
    ip, err := getOutboundIP()
    if err != nil {
        log.Fatalf("Failed to get IP address: %v", err)
    }

    // Register worker with retries
    maxRetries := 5
    for i := 0; i < maxRetries; i++ {
        if err := apiClient.Register(ip.String()); err != nil {
            if i == maxRetries-1 {
                log.Fatalf("Failed to register worker after %d attempts: %v", maxRetries, err)
            }
            log.Printf("Registration attempt %d failed: %v, retrying...", i+1, err)
            time.Sleep(time.Second * time.Duration(i+1))
            continue
        }
        break
    }

    log.Printf("Worker registered successfully with ID: %s", workerID)

    // Start heartbeat goroutine
    go func() {
        for {
            status := api.WorkerStatus{
                Services: map[string]api.ServiceStatus{
                    "dhcp": {Status: "running"},
                    "tftp": {Status: "running"},
                },
                Metrics: map[string]interface{}{
                    "memory_usage": 123456,
                    "cpu_usage": 5.2,
                    "active_leases": 10,
                },
            }

            if err := apiClient.SendHeartbeat(status); err != nil {
                log.Printf("Failed to send heartbeat: %v", err)
            }

            time.Sleep(30 * time.Second)
        }
    }()

    log.Printf("-------------------------------------------")
    log.Printf("Finished Registration Phase...")
    log.Printf("-------------------------------------------")
    // Init DB
    // invokeDB()

    // Invoke TFTP
    tc := tftpConfig{}
    tftpHandler(tc)
    
    // Invoke DHCP
    // dc := dhcpConfig{}
    // dc.bindAddr = "0.0.0.0"
    // dhcpHandler(dc)

}

func handler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {

    reply,err := dhcpv4.NewReplyFromRequest(m)
    if err != nil {
        log.Printf("NewReplyFrom Request failed: %v", err)
        return
    }
    
    // ------
    cidr := "10.70.1.0/24"
    offerIP, offerGateway, dbErr := invokeDB(cidr, m)

    if dbErr != nil {
        log.Printf("Failed to invoke DB handler: %v", dbErr)
        return
    }

    // Offer Values
    fmt.Println(reply)
    reply.YourIPAddr = net.ParseIP(offerIP)
    reply.ClientIPAddr = net.ParseIP(offerIP)
    reply.GatewayIPAddr = net.ParseIP(offerGateway)
    reply.ServerIPAddr = net.ParseIP(offerGateway)
    //reply.SubnetMask = net.IPMask{255,255,255,0}
    //reply.NumSeconds = 36000
    reply.IPAddressLeaseTime(36000)

    fmt.Println(m.SubnetMask)
    fmt.Println(reply.SubnetMask)
    
    reply.UpdateOption(dhcpv4.OptServerIdentifier(net.IP{10, 70, 1, 1}))
    reply.UpdateOption(dhcpv4.OptRouter(net.IP{10, 70, 1, 1}))
    reply.UpdateOption(dhcpv4.OptDNS(net.IP{1, 1, 1, 1}))
    reply.UpdateOption(dhcpv4.OptTFTPServerName("10.70.1.1"))
    //reply.UpdateOption(dhcpv4.OptTFTPServerIPAddress("ims-worker"))
    reply.UpdateOption(dhcpv4.OptSubnetMask(net.IPMask(net.IP{255,255,255,0})))
    reply.UpdateOption(dhcpv4.OptBootFileName("/var/lib/tftpboot/pxelinux.0"))
    //reply.UpdateOption(dhcpv4.IPAddressLeaseTime(3600))
    reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Second * 43200))
    fmt.Println(m.SubnetMask)
    fmt.Println(reply.SubnetMask)


    switch mt:= m.MessageType(); mt {
    case dhcpv4.MessageTypeDiscover:
        reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
        fmt.Println("Discover Mode ")
    case dhcpv4.MessageTypeRequest:
        fmt.Println("Request Mode ")
        fmt.Println("Reply Form")
        reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
    default:
        log.Printf("Unhandled Message type: %v", mt)
        return
    }

    fmt.Println(reply.Summary())
    if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
        log.Printf("Cannot reply to client: %v", err)
        return
    }
}

type logHook struct{}

func (h *logHook) OnSuccess(stats tftp.TransferStats) {
    fmt.Printf("Transfer of %s to %s complete\n", stats.Filename, stats.RemoteAddr)
}

func (h *logHook) OnFailure(stats tftp.TransferStats, err error) {
    fmt.Printf("Transfer of %s to %s failed: %v\n", stats.Filename, stats.RemoteAddr, err)
}

func dhcpHandler(dc dhcpConfig) {

    bindIP, err := netip.ParseAddr(dc.bindAddr)
    fmt.Println(bindIP)
    //fmt.Println(err)

    laddr := &net.UDPAddr{
                IP: net.ParseIP(dc.bindAddr),
                //Port: dhcpv4.DefaultServerPort,
                Port: 67,
    }

    server,err := server4.NewServer("dhcp-test", laddr, handler)
    if err != nil {
        log.Fatal(err)
    }
    
    server.Serve()

}

func tftpHandler(tc tftpConfig) {

    s := tftp.NewServer(tftpReadHandler, tftpWriteHandler)
    s.SetHook(&logHook{})
    go func() {
        //err := s.ListenAndServe(fmt.Sprintf(":%d", *port))
        err := s.ListenAndServe("0.0.0.0:69")
        if err != nil {
            fmt.Fprintf(os.Stdout, "Can't start the server: %v\n", err)
            os.Exit(1)
        }
    }()

    //time.Sleep(5000 * time.Minute)
    // START the DHCP server after tftp 
    dc := dhcpConfig{}
    dc.bindAddr = "0.0.0.0"
    dhcpHandler(dc)


    s.Shutdown()
}

// TFTP GET requests
func tftpReadHandler(filename string, rf io.ReaderFrom) error {
    file, err := os.Open(filename)

    if err != nil {
        fmt.Fprintf(os.Stderr, "Opening %s: %v\n", filename, err)
        return err
    }

    n,err := rf.ReadFrom(file)
    if err != nil {
        fmt.Fprintf(os.Stderr, "reading %s: %v\n", filename, err)
        return err
    }

    fmt.Printf("%d bytes sent\n", n)
    return nil
}

// TFTP PUT request
func tftpWriteHandler(filename string, wt io.WriterTo) error {
    file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Creating %s: %v\n", filename, err)
        return err
    }
    n, err := wt.WriteTo(file)
    if err != nil {
        fmt.Fprintf(os.Stderr, "writing %s: %v\n", filename, err)
        return err
    }
    fmt.Printf("%d bytes received\n", n)
    return nil
}

func invokeDB(cidr string, dhcpMessage *dhcpv4.DHCPv4) (string, string, error) {

    connStr := "postgres://nnag:password@10.70.1.1:54321/dhcpdb?sslmode=disable"

	// Initialize DB handler
	dbHandler, err := db.NewDBHandler(connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbHandler.Close()

	ctx := context.Background()


	// Add an IP pool
	// err = dbHandler.AddIPPool(ctx, "192.168.2.0/24", "192.168.2.1", "Test Pool")
	// if err != nil {
	// 	log.Printf("Error adding IP pool: %v", err)
	// }


	// Get all leases
	leases, err := dbHandler.GetLeases(ctx)
	if err != nil {
		log.Printf("Error retrieving leases: %v", err)
	} else {
		fmt.Println("Leases:")
		for _, l := range leases {
			fmt.Printf("- %s (%s): %s to %s [%s]\n",
				l.IPAddress, 
                l.MACAddress,
                l.LeaseStart,
                l.LeaseEnd,
                l.BindingState)
		}
	}

    // Get Available IP
    //cidr := "192.168.1.0/24"

    // Get IP Pool
    ipPools, err := dbHandler.GetIPPools(ctx)
	if err != nil {
		log.Printf("Error getting IP pool: %v", err)
	}
    
    var offerIP string
    var offerGateway string
    for _,  pool := range ipPools {
        if pool.CIDR == cidr {
            availableIP, err := dbHandler.GetAvailableIP(ctx, cidr)
            if err != nil {
                log.Fatal("Failed to find available IP: %v \n", err)
            }
            fmt.Printf("Available IP: %s\n", availableIP)
            offerIP = availableIP
            offerGateway = pool.Gateway
        }
    }

    for _,  pool := range ipPools {
            fmt.Printf("- ID: %d, CIDR: %s, Gateway %s, Description %s\n", pool.ID, pool.CIDR, pool.Gateway, pool.Description)
    }

    fmt.Println("------------------------------")
    fmt.Println(dhcpMessage.ClientHWAddr)
    fmt.Println("------------------------------")
	// Add a lease
	lease := db.Lease{
		IPAddress:        offerIP,
		MACAddress:       dhcpMessage.ClientHWAddr.String(),
		Hostname:         "test-device",
		LeaseStart:       time.Now(),
		LeaseEnd:         time.Now().Add(1 * time.Hour),
		BindingState:     "active",
		LastTransaction:  time.Now(),
		NextBindingState: "expired",
		BootfileURL:      "/path/to/bootfile",
		TFTPServer:       offerGateway,
		IPPoolID:         1,
	}

	err = dbHandler.AddLease(ctx, lease)
	if err != nil {
		log.Printf("Error adding lease: %v", err)
	}

    return offerIP, offerGateway, err

}

func getOutboundIP() (net.IP, error) {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        return nil, err
    }
    defer conn.Close()

    localAddr := conn.LocalAddr().(*net.UDPAddr)
    return localAddr.IP, nil
}
