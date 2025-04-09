package main

import (
        "time"
        "fmt"
        "net"
        "log"
        "github.com/insomniacslk/dhcp/dhcpv4"
        "github.com/insomniacslk/dhcp/dhcpv4/server4"

        "io/ioutil"
        "path/filepath"

        "strings"

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
    serverIP string  // Add server IP for consistent responses
    bootFilePath string // Add boot file path configuration
}

type tftpConfig struct {
    enabled bool
    bindAddr string
    bindPort int
    blockSize int
    rootDir string // Add TFTP root directory
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

            machines, err := apiClient.GetMachines()
            if err != nil {
                log.Printf("Failed to get machines: %v", err)
                continue
            }
            for _, machine := range machines {
                fmt.Printf("Machine: %s (IP: %s, Cluster: %s, Status: %s)\n", 
                    machine.Hostname, machine.IP, machine.ClusterName, machine.Status)
            }

            time.Sleep(10 * time.Second)
            fmt.Printf("---------------------------------------------\n")
        }
    }()

    log.Printf("-------------------------------------------")
    log.Printf("Finished Registration Phase...")
    log.Printf("-------------------------------------------")

    // Set up the server IP - this should be your actual server IP
    //serverIP := ip.String() // Using outbound IP by default
    serverIP := "10.80.1.1"

    if err != nil {
        log.Fatalf("Failed to get IP address: %v", err)
    }
    log.Printf("Server detected IP: %s", serverIP)
    
    // Set up the configurations
    tftpRootDir := "/var/lib/tftpboot"
    bootFileName := "pxelinux.0"
    
    // Verify TFTP directory and critical files exist
    if _, err := os.Stat(tftpRootDir); os.IsNotExist(err) {
        log.Fatalf("TFTP root directory does not exist: %s", tftpRootDir)
    }
    
    pxelinuxPath := filepath.Join(tftpRootDir, bootFileName)
    if _, err := os.Stat(pxelinuxPath); os.IsNotExist(err) {
        log.Fatalf("PXE boot file not found: %s", pxelinuxPath)
    }
    
    // Setup configurations
    dc := dhcpConfig{
        enabled: true,
        bindAddr: "0.0.0.0",
        serverIP: serverIP,
        bootFilePath: bootFileName, // Just the filename, no leading slash
    }

    tc := tftpConfig{
        enabled: true,
        bindAddr: "0.0.0.0",
        bindPort: 69,
        rootDir: tftpRootDir,
    }

    // Start TFTP server first
    log.Printf("Initializing TFTP server...")
    go tftpHandler(tc)
    
    // Give TFTP server time to initialize
    time.Sleep(2 * time.Second)
    
    // Then start DHCP server
    log.Printf("Initializing DHCP server...")
    dhcpHandler(dc)

}

type StringOption string
func (o StringOption) String() string {
    return string(o)
}

func handler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4, config dhcpConfig) {
    // Log the incoming request
    log.Printf("Received DHCP request from %s, type: %s", m.ClientHWAddr.String(), m.MessageType())

    // Check is this request from old ass device
    isLegacyPXE := isLegacyPXEClient(m)
    if isLegacyPXE {
        log.Printf("DHCP: Detected legacy PXE client: %s", m.ClientHWAddr.String())
    }
    
    reply, err := dhcpv4.NewReplyFromRequest(m)


    // LegacyPXE 
    if isLegacyPXE {
        reply.SetBroadcast()
        log.Printf("DHCP: Using broadcast response for legacy PXE client")
    } else {
        // Use unicast for modern clients
        reply.SetUnicast()
        log.Printf("DHCP: Using unicast response for client")
    }

    if err != nil {
        log.Printf("NewReplyFromRequest failed: %v", err)
        return
    }

    // Get the fixed server IP (should be set in config)
    serverIP := net.ParseIP("10.80.1.1")
    if serverIP == nil {
        log.Printf("Invalid server IP: %s", config.serverIP)
        return
    }
    
    // CIDR for the subnet
    cidr := "10.80.1.0/24"

    // Check if MAC already has a lease
    ctx := context.Background()
    offerIP, offerGateway, exists, dbErr := checkExistingLease(ctx, cidr, m)
    
    if dbErr != nil {
        log.Printf("Failed to check lease: %v", dbErr)
        return
    }
    
    if !exists {
        // Get a new IP if no existing lease
        offerIP, offerGateway, dbErr = invokeDB(cidr, m)
        if dbErr != nil {
            log.Printf("Failed to invoke DB handler: %v", dbErr)
            return
        }
    }

    log.Printf("Offering IP %s to client %s (Gateway: %s)", offerIP, m.ClientHWAddr.String(), offerGateway)

    // Offer Values
    reply.YourIPAddr = net.ParseIP("10.80.1.10") // Temporary IP
    reply.ServerIPAddr = serverIP
    reply.GatewayIPAddr = net.ParseIP(offerGateway)
    
    // PXE boot requires specific fields
    if isPXEClient(m) {
        log.Printf("PXE client detected: %s", m.ClientHWAddr)
        
        // Use consistent server IP for DHCP and TFTP
        bootServer := serverIP
        
        // Make boot filename relative to TFTP root and ensure it exists
        bootFileName := config.bootFilePath

        // legacy dumb ass
        //reply.NextServerIPAddr = serverIP

        
        // PXE specific options
        //reply.ServerHostName = "10.80.1.1"
        reply.BootFileName = bootFileName
        
        // Server identifier must be constant
        reply.UpdateOption(dhcpv4.OptServerIdentifier(serverIP))
        
        // Network config
        reply.UpdateOption(dhcpv4.OptSubnetMask(net.IPMask(net.IP{255, 255, 255, 0})))
        reply.UpdateOption(dhcpv4.OptRouter(net.ParseIP(offerGateway)))
        reply.UpdateOption(dhcpv4.OptDNS(net.IP{8, 8, 8, 8}))
        
        // PXE options
        reply.UpdateOption(dhcpv4.OptTFTPServerName(bootServer.String()))
        reply.UpdateOption(dhcpv4.OptBootFileName(bootFileName))
        
        // Vendor specific for PXE
        //reply.UpdateOption(dhcpv4.OptClassIdentifier("PXEClient"))

    } else {
        // Standard DHCP options for non-PXE clients
        reply.UpdateOption(dhcpv4.OptServerIdentifier(serverIP))
        reply.UpdateOption(dhcpv4.OptSubnetMask(net.IPMask(net.IP{255, 255, 255, 0})))
        reply.UpdateOption(dhcpv4.OptRouter(net.ParseIP(offerGateway)))
        reply.UpdateOption(dhcpv4.OptDNS(net.IP{8, 8, 8, 8}))
    }
    
    // Set lease time
    reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Hour * 12))
    
    // Set appropriate response type
    switch mt := m.MessageType(); mt {
    case dhcpv4.MessageTypeDiscover:
        log.Printf("DHCP Discover from %s", m.ClientHWAddr)
        reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
    case dhcpv4.MessageTypeRequest:
        log.Printf("DHCP Request from %s", m.ClientHWAddr)
        reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
    default:
        log.Printf("Unhandled Message type: %v", mt)
        return
    }

    // Log the reply we're sending
    log.Printf("Sending reply: %s", reply.Summary())

    // Send the response
    if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
        log.Printf("Cannot reply to client: %v", err)
        return
    }
}

// isPXEClient checks if the client is requesting PXE boot
func isPXEClient(m *dhcpv4.DHCPv4) bool {
    // Check for PXE client identifier
    vendorClass := m.Options.Get(dhcpv4.OptionClassIdentifier)
    if vendorClass != nil && (string(vendorClass) == "PXEClient" || string(vendorClass) == "PXEClient:Arch:00000:UNDI:002001") {
        return true
    }
    
    // Check for Option 93 (Client System Architecture)
    archType := m.Options.Get(dhcpv4.OptionClientSystemArchitectureType)
    if archType != nil {
        return true
    }
    
    // Check for Option 94 (UUID/GUID)
    clientUUID := m.Options.Get(dhcpv4.OptionClientNetworkInterfaceIdentifier)
    if clientUUID != nil {
        return true
    }
    
    return false
}

func isLegacyPXEClient(m *dhcpv4.DHCPv4) bool {
    // Check vendor class identifier
    vendorClass := m.Options.Get(dhcpv4.OptionClassIdentifier)
    if vendorClass != nil {
        vcString := string(vendorClass)
        // These patterns often indicate legacy PXE clients
        legacyPatterns := []string{
            "PXEClient:Arch:00000", // Very old PXE
            "PXEClient:Arch:00006", // IA32 BIOS PXE clients (most legacy clients)
            "PXEClient:Arch:00007", // x86-64 BIOS PXE clients (some older x64 systems)
        }
        
        for _, pattern := range legacyPatterns {
            if strings.Contains(vcString, pattern) {
                return true
            }
        }
    }
    
    // If client has no IP and requests broadcast, likely legacy
    if m.ClientIPAddr.IsUnspecified() && m.IsBroadcast() {
        return true
    }
    
    // Client system architecture type can also indicate legacy clients
    archType := m.Options.Get(dhcpv4.OptionClientSystemArchitectureType)
    if archType != nil && len(archType) >= 2 {
        // Check if architecture type is legacy BIOS PC (type 0)
        if archType[0] == 0 && archType[1] == 0 {
            return true
        }
    }
    
    return false
}

// checkExistingLease checks if a MAC already has a lease and returns it
func checkExistingLease(ctx context.Context, cidr string, dhcpMessage *dhcpv4.DHCPv4) (string, string, bool, error) {
    connStr := "postgres://nnag:password@10.70.1.1:54321/dhcpdb?sslmode=disable"

    // Initialize DB handler
    dbHandler, err := db.NewDBHandler(connStr)
    if err != nil {
        return "", "", false, fmt.Errorf("failed to connect to database: %v", err)
    }
    defer dbHandler.Close()

    // Get IP pool for gateway info
    ipPools, err := dbHandler.GetIPPools(ctx)
    if err != nil {
        return "", "", false, fmt.Errorf("error getting IP pools: %v", err)
    }
    
    var gateway string
    for _, pool := range ipPools {
        if pool.CIDR == cidr {
            gateway = pool.Gateway
            break
        }
    }
    
    if gateway == "" {
        return "", "", false, fmt.Errorf("no gateway found for CIDR %s", cidr)
    }

    // Check if MAC already has a lease
    macAddress := dhcpMessage.ClientHWAddr.String()
    lease, err := dbHandler.GetLeaseByMAC(ctx, macAddress)
    if err != nil {
        // If it's a "not found" error, return false
        return "", gateway, false, nil
    }
    
    // Check if lease is still valid
    if lease.LeaseEnd.After(time.Now()) {
        return lease.IPAddress, gateway, true, nil
    }
    
    // Lease expired, return false
    return "", gateway, false, nil
}

type logHook struct{}

func (h *logHook) OnSuccess(stats tftp.TransferStats) {
    fmt.Printf("Transfer of %s to %s complete\n", stats.Filename, stats.RemoteAddr)
    //fmt.Printf("Transfer of %s to %s complete (%d bytes)\n", stats.Filename, stats.RemoteAddr, stats.BytesTransferred)
}

func (h *logHook) OnFailure(stats tftp.TransferStats, err error) {
    fmt.Printf("Transfer of %s to %s failed: %v\n", stats.Filename, stats.RemoteAddr, err)
}

func dhcpHandler(dc dhcpConfig) {
    laddr := &net.UDPAddr{
        IP:   net.ParseIP(dc.bindAddr),
        Port: 67,
    }

    // Create a handler function that includes the config
    handlerWithConfig := func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
        handler(conn, peer, m, dc)
    }

    // Use a specific interface if provided, otherwise use "any" interface
    //interfaceName := "dhcp-test"
    interfaceName := "enp0s20f0u4"
    if dc.bindInterface != "" {
        interfaceName = dc.bindInterface
    }

    log.Printf("Starting DHCP server on %s:%d (interface: %s)", dc.bindAddr, 67, interfaceName)
    
    server, err := server4.NewServer(interfaceName, laddr, handlerWithConfig)
    if err != nil {
        log.Fatalf("Failed to create DHCP server: %v", err)
    }
    
    log.Println("DHCP server started. Waiting for requests...")
    if err := server.Serve(); err != nil {
        log.Fatalf("DHCP server error: %v", err)
    }
}

func tftpHandler(tc tftpConfig) {
    // Ensure TFTP root directory exists
    if _, err := os.Stat(tc.rootDir); os.IsNotExist(err) {
        log.Fatalf("TFTP root directory does not exist: %s", tc.rootDir)
    }
    
    // Set TFTP read callback with root directory
    readHandler := func(filename string, rf io.ReaderFrom) error {
        // Normalize path by removing leading slash if present
        if filename[0] == '/' {
            filename = filename[1:]
        }
        
        // Prepend root directory to filename
        fullPath := filepath.Join(tc.rootDir, filename)
        
        log.Printf("TFTP READ REQUEST: Client requested file: %s (full path: %s)", filename, fullPath)
        
        // Check if the file exists
        if _, err := os.Stat(fullPath); os.IsNotExist(err) {
            log.Printf("ERROR: File not found: %s", fullPath)
            return fmt.Errorf("file not found: %s", filename)
        }
        
        file, err := os.Open(fullPath)
        if err != nil {
            log.Printf("TFTP ERROR: Failed to open %s: %v", fullPath, err)
            return err
        }
        defer file.Close()

        // Get file size for logging
        fileInfo, _ := file.Stat()
        fileSize := fileInfo.Size()
        log.Printf("TFTP: Sending file %s (size: %d bytes)", filename, fileSize)
        
        n, err := rf.ReadFrom(file)
        if err != nil {
            log.Printf("TFTP ERROR: Failed reading %s: %v", fullPath, err)
            return err
        }

        log.Printf("TFTP SUCCESS: %d bytes sent for %s", n, filename)
        return nil
    }
    
    // Set TFTP write callback with root directory
    writeHandler := func(filename string, wt io.WriterTo) error {
        // Normalize path
        if filename[0] == '/' {
            filename = filename[1:]
        }
        
        // Prepend root directory to filename
        fullPath := filepath.Join(tc.rootDir, filename)
        
        log.Printf("TFTP WRITE REQUEST: %s (full path: %s)", filename, fullPath)
        
        // Ensure parent directory exists
        err := os.MkdirAll(filepath.Dir(fullPath), 0755)
        if err != nil {
            log.Printf("TFTP ERROR: Failed to create directory for %s: %v", fullPath, err)
            return err
        }
        
        file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
        if err != nil {
            log.Printf("TFTP ERROR: Failed to create %s: %v", fullPath, err)
            return err
        }
        defer file.Close()
        
        n, err := wt.WriteTo(file)
        if err != nil {
            log.Printf("TFTP ERROR: Failed writing to %s: %v", fullPath, err)
            return err
        }

        log.Printf("TFTP SUCCESS: %d bytes received for %s", n, filename)
        return nil
    }

    // Create a new TFTP server with our handlers
    s := tftp.NewServer(readHandler, writeHandler)
    s.SetHook(&logHook{})
    
    // Configure TFTP options
    s.SetTimeout(5 * time.Second)  // Set connection timeout
    
    // Set block size if specified (default is 512)
    if tc.blockSize > 0 {
        s.SetBlockSize(tc.blockSize)
    }
    
    addr := fmt.Sprintf("%s:%d", tc.bindAddr, tc.bindPort)
    log.Printf("Starting TFTP server on %s (root directory: %s)", addr, tc.rootDir)
    
    // Verify TFTP root directory is accessible
    log.Printf("Checking TFTP root directory: %s", tc.rootDir)
    files, err := ioutil.ReadDir(tc.rootDir)
    if err != nil {
        log.Fatalf("Cannot access TFTP root directory: %v", err)
    }
    
    // Log files in TFTP root for debugging
    log.Printf("Files in TFTP root:")
    for _, file := range files {
        log.Printf("  - %s (%d bytes)", file.Name(), file.Size())
    }
    
    // Check for pxelinux.0 specifically
    pxelinuxPath := filepath.Join(tc.rootDir, "pxelinux.0")
    if _, err := os.Stat(pxelinuxPath); os.IsNotExist(err) {
        log.Printf("WARNING: pxelinux.0 not found at %s", pxelinuxPath)
    } else {
        log.Printf("Found pxelinux.0 at %s", pxelinuxPath)
    }
    
    // Start the TFTP server
    err = s.ListenAndServe(addr)
    if err != nil {
        log.Fatalf("TFTP server error: %v", err)
    }
}


func invokeDB(cidr string, dhcpMessage *dhcpv4.DHCPv4) (string, string, error) {
    connStr := "postgres://nnag:password@10.70.1.1:54321/dhcpdb?sslmode=disable"

    // Initialize DB handler
    dbHandler, err := db.NewDBHandler(connStr)
    if err != nil {
        return "", "", fmt.Errorf("failed to connect to database: %v", err)
    }
    defer dbHandler.Close()

    ctx := context.Background()

    // Get all IP pools
    ipPools, err := dbHandler.GetIPPools(ctx)
    if err != nil {
        return "", "", fmt.Errorf("error getting IP pools: %v", err)
    }
    
    var offerIP string
    var offerGateway string

    // Find matching pool and get available IP
    for _, pool := range ipPools {
        if pool.CIDR == cidr {
            availableIP, err := dbHandler.GetAvailableIP(ctx, cidr)
            if err != nil {
                return "", "", fmt.Errorf("failed to find available IP: %v", err)
            }
            log.Printf("Available IP: %s", availableIP)
            offerIP = availableIP
            offerGateway = pool.Gateway
        }
    }

    if offerIP == "" || offerGateway == "" {
        return "", "", fmt.Errorf("no available IP or gateway found for CIDR %s", cidr)
    }

    log.Printf("Offering IP %s (Gateway: %s) to client %s", offerIP, offerGateway, dhcpMessage.ClientHWAddr)
    
    // Add a lease
    hostname := dhcpMessage.HostName()
    if hostname == "" {
        hostname = "pxe-" + dhcpMessage.ClientHWAddr.String()
    }
    
    lease := db.Lease{
        IPAddress:        offerIP,
        MACAddress:       dhcpMessage.ClientHWAddr.String(),
        Hostname:         hostname,
        LeaseStart:       time.Now(),
        LeaseEnd:         time.Now().Add(12 * time.Hour),
        BindingState:     "active",
        LastTransaction:  time.Now(),
        NextBindingState: "expired",
        BootfileURL:      "/pxelinux.0",
        TFTPServer:       offerGateway,
        IPPoolID:         1,
    }

    err = dbHandler.AddLease(ctx, lease)
    if err != nil {
        return "", "", fmt.Errorf("error adding lease: %v", err)
    }

    return offerIP, offerGateway, nil
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

