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
		"flag"
        "io"
        "os"
        "github.com/pin/tftp/v3"
        "context"
		"strconv"
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
    enabled       bool
    mode          string
    bindAddr      string
    bindInterface string
    tftpIP        string
    tftpPort      int
    serverIP      string // Add server IP for consistent responses
    bootFilePath  string // Add boot file path configuration
}

type tftpConfig struct {
    enabled   bool
    bindAddr  string
    bindPort  int
    blockSize int
    rootDir   string // Add TFTP root directory
}



func main() {

	// Kubernetes Related declaration
	var (
		kubeconfig = flag.String("Kubeconfig", "", "/root/.kube/config")
		insecure = flag.Bool("insecure", false, "skip TLS Verification")
		//----------------------------------------------------------------------------------------------------------------
		//prometheusNamespace = flag.String("prom-namespace", "observability", "namespace where prometheus is deployed")
		//prometheusService = flag.String("prom-service","prometheus-operated","Prometheus service name")
		//targetNamespace = flag.String("namespace","default","namespace to query metrics for")
		//queryType = flag.String("query","nodes","type of query: nodes, pods, custom")
		//customQuery = flag.String("custom-qyery", "", "custom PromQL query")
		//----------------------------------------------------------------------------------------------------------------
	)
	flag.Parse()
	
	// Configure fetcher
	opts := &Options{
		KubeConfig: *kubeconfig,
		HTTPTimeout: 30 * time.Second,
		InsecureSkipVerify: *insecure,
	}

	// Create new metrics fetcher
	fetcher, err := NewMetricsFetcher(opts)
	if err != nil {
		log.Fatalf("Failed to creat metrics fetcher: %v", err)
	}

	defer fetcher.Close()

    //----------------------------------------------------------------------
    // Get worker ID (could be hostname or container ID)
    workerID, err := os.Hostname()
    if err != nil {
        log.Fatalf("Failed to get hostname: %v", err)
    }

    // Get credentials from environment variables
    username := os.Getenv("IMS_API_USERNAME")
    password := os.Getenv("IMS_API_PASSWORD")
    apiURL := os.Getenv("IMS_API_URL")

	envIMSMode := os.Getenv("IMS_MODE")
	enableIMSMode, err := strconv.ParseBool(envIMSMode)

	if err != nil {
		enableIMSMode = false
	}

	if enableIMSMode == true {
    	if username == "" || password == "" || apiURL == "" {
    	    log.Fatal("IMS_API_USERNAME, IMS_API_PASSWORD and IMS_API_URL environment variables are required")
    	}
	}

	// New: Get metrics URL from environment or use default
	metricsURL := os.Getenv("IMS_METRICS_URL")
	metricsID := 0
	if metricsURL == "" {
		//metricsURL = apiURL + "/api/machines/21/update_metrics/" // Default endpoint
		// metrics ID should be dynamic dictacted from the IMS
		metricsURL = apiURL + "/api/clusters/" + strconv.Itoa(metricsID) + "/update_metrics/" // Default endpoint
	}

    // Create API client with authentication
    apiClient := NewAPIClient(apiURL, workerID, username, password)

    // Get IP address
    ip, err := getOutboundIP()
    if err != nil {
        log.Fatalf("Failed to get IP address: %v", err)
    }

    // Register worker with retries
	if enableIMSMode == true {
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
		authToken := apiClient.GetAuthHeader()
		log.Printf("AuthToken: %s\n", authToken)
	}

	metricsCollector := NewCollector(workerID, metricsURL, username, password, apiClient)
	//ctx := context.Background()
    // Start heartbeat goroutine
    go func() {
        for {
			// New: Create metrics collector
            // NEW: Add metrics to heartbeat
            metricCounts := metricsCollector.GetMetricCounts()

			// Memory Info
			memUsed,memTotal, err := systemGetMemoryUsage()
			if err != nil {
				fmt.Printf("Getting Memory error: %v", err)
			}

			// CPU Info
			cpuPercentage,err := systemGetCPUUsage()
			if err != nil {
				fmt.Printf("Getting Memory error: %v", err)
			}

			fmt.Printf("\nMem: %f Used of %f Total\n", memUsed, memTotal)
			fmt.Printf("CPU: %f% \n", cpuPercentage)
            status := WorkerStatus{
                Services: map[string]ServiceStatus{
                    "dhcp": {Status: "running"},
                    "tftp": {Status: "running"},
                },
                Metrics: map[string]interface{}{
					// Find Real usecase 
                    "memory_usage": memUsed, // in GB
                    "cpu_usage": cpuPercentage, // Percentage
                    "active_leases": 10,
                    // NEW: Add metric counts to status
                    "metrics_collected": len(metricCounts),
                },
            }

			// New: Include metrics counts in heartbeat
			for metricType, count := range metricCounts {
				status.Metrics[metricType] = count
			}
			if enableIMSMode == true {
            	if err := apiClient.SendHeartbeat(status); err != nil {
            	    log.Printf("Failed to send heartbeat: %v", err)
            	}

            	machines, err := apiClient.GetMachines()
            	if err != nil {
            	    log.Printf("Failed to get machines: %v", err)
            	    continue
            	}
            	for _, machine := range machines {
					fmt.Printf("ID: %d, Machine: %s (IP: %s, Cluster: %s, Status: %s)\n", 
            	        machine.ID, machine.Hostname, machine.IP, machine.ClusterName, machine.Status)

            	    // NEW: Record metric for machine processing
            	    metricsCollector.RecordMetric("pxe.machine.processed", 1, machine.MAC, machine.IP, map[string]string{
            	        "hostname": machine.Hostname,
            	        "status": machine.Status,
            	    })
            	    updateDB(machine)

            	}
			}



            // NEW: Ensure we report any buffered metrics
            metricsCollector.ReportMetrics()

            // fmt.Printf("*****************[ K8S Stats ]*******************\n")
	    //     	//k8sRet, err := GetKubeNode()
	    //     	//fmt.Printf("Kubernetes status: %s", k8sRet)

	    //     	switch *queryType {
	    //     	case "nodes": 
	    //     		if err := queryNodeMetrics(ctx, fetcher, *prometheusNamespace, *prometheusService); err != nil {
	    //     			log.Fatalf("Failed to query node metrics: %v", err)
	    //     		}
	    //     	}

	    //     	fmt.Printf("*************************************************\n")

            // time.Sleep(10 * time.Second)
            // fmt.Printf("---------------------------------------------\n")
        }
    }()

    log.Printf("-------------------------------------------")
    log.Printf("Finished Registration Phase...")
    log.Printf("-------------------------------------------")

    // Set up the server IP - this should be your actual server IP
    //serverIP := ip.String() // Using outbound IP by default

    // if err != nil {
    //     log.Fatalf("Failed to get IP address: %v", err)
    // }
    // log.Printf("Server detected IP: %s", serverIP)
    
    // Set up the configurations
	tftpRootDir := os.Getenv("TFTP_PATH")
    bootFileName := "pxelinux.0"
    
    // Verify TFTP directory and critical files exist
    if _, err := os.Stat(tftpRootDir); os.IsNotExist(err) {
        log.Fatalf("TFTP root directory does not exist: %s", tftpRootDir)
    }
    
    pxelinuxPath := filepath.Join(tftpRootDir, bootFileName)
    if _, err := os.Stat(pxelinuxPath); os.IsNotExist(err) {
        log.Fatalf("PXE boot file not found: %s", pxelinuxPath)
    }
    serverIP := os.Getenv("SERVER_IP")
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
	go tftpHandler(tc, metricsCollector)
    
    // Give TFTP server time to initialize
    time.Sleep(2 * time.Second)
    
    // Then start DHCP server
    log.Printf("Initializing DHCP server...")
	go dhcpHandler(dc, metricsCollector)

	// Wait indefinitely
	wait := make(chan struct{})
	<-wait

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
    serverIP := net.ParseIP(os.Getenv("SERVER_IP"))

    if serverIP == nil {
        log.Printf("Invalid server IP: %s", config.serverIP)
        return
    }
    
    // CIDR for the subnet
    //cidr := "10.80.1.0/24"
	cidr := os.Getenv("CIDR")


    // Check if MAC already has a lease
    ctx := context.Background()
    offerIP, offerGateway, offerBootfile, exists, dbErr := checkExistingLease(ctx, cidr, m)
    if dbErr != nil {
        log.Printf("Failed to check lease: %v", dbErr)
        return
    }

    if !exists {
        // Uncomment this routine to allow non registered Device into our system
        // Get a new IP if no existing lease
        // offerIP, offerGateway, dbErr = invokeDB(cidr, m)
        // if dbErr != nil {
        //     log.Printf("Failed to invoke DB handler: %v", dbErr)
        //     return
        // }
        log.Printf("Unknown Device detected")
        return
    } else {

        // Update DB When found: Lease Time
        handlerConn := "postgres://nnag:password@localhost:54321/dhcpdb?sslmode=disable"
        updateRet, dbStatus := invokeDBUpdate(handlerConn, m.ClientHWAddr.String())

        if dbStatus != nil {
            log.Printf("Lease Status error %v", updateRet)
            return
        }

        log.Printf("Offering IP %s to client %s (Gateway: %s)", offerIP, m.ClientHWAddr.String(), offerGateway)

        // Offer Values
        reply.YourIPAddr = net.ParseIP(offerIP) // Temporary IP
        reply.ServerIPAddr = serverIP
        reply.GatewayIPAddr = net.ParseIP(offerGateway)
        
        // PXE boot requires specific fields
        if isPXEClient(m) {
            log.Printf("PXE client detected: %s", m.ClientHWAddr)
            
            // Use consistent server IP for DHCP and TFTP
            bootServer := serverIP
            
            // Make boot filename relative to TFTP root and ensure it exists
            log.Printf("---------------------------")
            log.Printf("Offerbootfile: %s", offerBootfile)
            log.Printf("---------------------------")
            bootFileName := config.bootFilePath

            // legacy dumb ass
            //reply.NextServerIPAddr = serverIP
            
            // PXE specific options
            reply.ServerHostName = offerGateway
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
            
            // Vendor specific for PXE islegacyPXE function is not 
            // working properly deal with it later
            //
            // if (isLegacyPXE) {
            //     log.Printf("Client is Not LegacyPXE applying Class Identifier")
            //     reply.UpdateOption(dhcpv4.OptClassIdentifier("PXEClient"))
            // }

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

        // Print IP Pools and Leases 
        leaseStatus, dbErr :=  checkLeases(ctx, cidr)
        if dbErr != nil {
            log.Printf("Lease Status error %v", leaseStatus)
            return
        }
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
func checkExistingLease(ctx context.Context, cidr string, dhcpMessage *dhcpv4.DHCPv4) (string, string, string, bool, error) {
    connStr := "postgres://nnag:password@localhost:54321/dhcpdb?sslmode=disable"

    // Initialize DB handler
    dbHandler, err := NewDBHandler(connStr)
    if err != nil {
        return "", "", "", false, fmt.Errorf("failed to connect to database: %v", err)
    }

    defer dbHandler.Close()

    // Get IP pool for gateway info
    ipPools, err := dbHandler.GetIPPools(ctx)
    if err != nil {
        return "", "", "", false, fmt.Errorf("error getting IP pools: %v", err)
    }
    
    var gateway string
    for _, pool := range ipPools {
        if pool.CIDR == cidr {
            gateway = pool.Gateway
            break
        }
    }
    
    if gateway == "" {
        return "", "", "", false, fmt.Errorf("no gateway found for CIDR %s", cidr)
    }

    // Check if MAC already has a lease
    macAddress := dhcpMessage.ClientHWAddr.String()
    lease, err := dbHandler.GetLeaseByMAC(ctx, macAddress)
    //offerIP, offerGateway, exists, dbErr := checkExistingLease(ctx, cidr, m)

    if err != nil {
        // If it's a "not found" error, return false
        return "", gateway, lease.BootfileURL, false, nil
    }
    
    // Check if lease is still valid
    if lease.LeaseEnd.After(time.Now()) {
        return lease.IPAddress, gateway, lease.BootfileURL,  true, nil
    }
    
    // Lease expired, return false
    return "", gateway, lease.BootfileURL, false, nil

}

func checkLeases(ctx context.Context, cidr string) (bool, error) {

    connStr := "postgres://nnag:password@10.70.1.1:54321/dhcpdb?sslmode=disable"

    dbHandler, err := NewDBHandler(connStr)
    if err != nil {
        return false, fmt.Errorf("failed to connect to database: %v", err)
    }

    defer dbHandler.Close()

    // Get IP pool for gateway info
    ipPools, err := dbHandler.GetIPPools(ctx)
    if err != nil {
        return false, fmt.Errorf("error getting IP pools: %v", err)
    }

    fmt.Println("Pool Lists:")
    for _, pool := range ipPools {
        fmt.Printf("%d, %s, %s, %s\n", pool.ID, pool.CIDR, pool.Gateway, pool.Description)
    }

    // Get IP pool for gateway info
    leases, err := dbHandler.GetLeases(ctx)

    if err != nil {
        return false, fmt.Errorf("error getting IP Leases: %v", err)
    }

    fmt.Println("[Pool Lists]")
    for _, lease := range leases {
        fmt.Printf("%s, %s, %s, %s, %s, %s\n",
            lease.IPAddress,
            lease.MACAddress,
            lease.BindingState,
            lease.Hostname,
            lease.BootfileURL,
            lease.LastTransaction)
    }

    return true, nil

}

type logHook struct{}

type transferStats struct {
	Filename string
	RemoteAddr net.UDPAddr
	BytesTransferred int64
}

func (h *logHook) OnSuccess(stats tftp.TransferStats) {
    fmt.Printf("Transfer of %s to %s complete\n", stats.Filename, stats.RemoteAddr)
    //fmt.Printf("Transfer of %s to %s complete (%d bytes)\n", stats.Filename, stats.RemoteAddr, stats.BytesTransferred)
}

func (h *logHook) OnFailure(stats tftp.TransferStats, err error) {
    fmt.Printf("Transfer of %s to %s failed: %v\n", stats.Filename, stats.RemoteAddr, err)
}

func dhcpHandler(dc dhcpConfig, metricsCollector *Collector) {

    laddr := &net.UDPAddr{
        IP:   net.ParseIP(dc.bindAddr),
        Port: 67,
    }

    // Create a handler function that includes metrics
    handlerWithMetrics := func(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
        // Record metrics BEFORE processing
        if metricsCollector != nil {
            switch mt := m.MessageType(); mt {
            case dhcpv4.MessageTypeDiscover:
                metricsCollector.RecordDHCPEvent(MetricTypeDHCPDiscoverCount, m.ClientHWAddr.String(), "", 1)
            case dhcpv4.MessageTypeRequest:
                metricsCollector.RecordDHCPEvent(MetricTypeDHCPRequestCount, m.ClientHWAddr.String(), "", 1)
            }
        }
        
        // Call original handler
        handler(conn, peer, m, dc)
        
        // Record metrics AFTER processing - assuming success
        if metricsCollector != nil {
            switch mt := m.MessageType(); mt {
            case dhcpv4.MessageTypeDiscover:
                metricsCollector.RecordDHCPEvent(MetricTypeDHCPOfferCount, m.ClientHWAddr.String(), "", 1)
            case dhcpv4.MessageTypeRequest:
                metricsCollector.RecordDHCPEvent(MetricTypeDHCPAckCount, m.ClientHWAddr.String(), "", 1)
            }
        }
    }

    // Rest of your original code
    //interfaceName := "enp0s20f0u4"
    //interfaceName := "enp2s0"
    interfaceName := os.Getenv("PXE_INTERFACE")
    fmt.Println("Using port ", interfaceName)

    if dc.bindInterface != "" {
        interfaceName = dc.bindInterface
    }

    log.Printf("Starting DHCP server on %s:%d (interface: %s)", dc.bindAddr, 67, interfaceName)
    
    server, err := server4.NewServer(interfaceName, laddr, handlerWithMetrics)
    if err != nil {
        log.Fatalf("Failed to create DHCP server: %v", err)
    }
    
    log.Println("DHCP server started. Waiting for requests...")
    if err := server.Serve(); err != nil {
        log.Fatalf("DHCP server error: %v", err)
    }

}

func tftpHandler(tc tftpConfig, metricsCollector *Collector) {
	// Log existing hook
	originalHook := &logHook{}

	//
    // Ensure TFTP root directory exists
	//
    if _, err := os.Stat(tc.rootDir); os.IsNotExist(err) {
        log.Fatalf("TFTP root directory does not exist: %s", tc.rootDir)
    }
    
	// Create TFTP metrics hook
	metricsHook := NewTFTPMetricsHook(metricsCollector, tc.rootDir)

    // Set TFTP read callback with root directory
    readHandler := metricsHook.ReadHandler(func(filename string, rf io.ReaderFrom) error {
        // Your original read handler logic
        
        // Normalize path by removing leading slash if present
        if filename[0] == '/' {
            filename = filename[1:]
        }
        
        // Prepend root directory to filename
        fullPath := filepath.Join(tc.rootDir, filename)
        
        log.Printf("TFTP READ REQUEST: Client requested file: %s (full path: %s)", filename, fullPath)
        
        // Extract client IP for logging
        remoteAddr := rf.(tftp.OutgoingTransfer).RemoteAddr()
        clientIPPort := remoteAddr.String()
        clientIP := clientIPPort[:strings.LastIndex(clientIPPort, ":")]
        
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
        
        // Set file size if available in your TFTP library
        if setter, ok := rf.(interface{ SetSize(int64) }); ok {
            setter.SetSize(fileSize)
        }
        
        startTime := time.Now()
        n, err := rf.ReadFrom(file)
        duration := time.Since(startTime)
        
        if err != nil {
            log.Printf("TFTP ERROR: Failed reading %s: %v", fullPath, err)
            return err
        }

        log.Printf("TFTP SUCCESS: %d bytes sent for %s in %v", n, filename, duration)
        
        // Call the metrics hook's success callback directly
        metricsLogHook := NewMetricsLogHook(metricsCollector, originalHook)
        metricsLogHook.OnSuccess(clientIP, filename, n, duration)
        
        // Invoke the original logHook's OnSuccess
        // originalHook.OnSuccess(transferStats{
        //     Filename: filename,
        //     RemoteAddr: remoteAddr,
        //     BytesTransferred: n,
        // })
		//
        
        return nil
    })
    
    // Set TFTP write callback with root directory
	    // Set TFTP write callback with root directory
    writeHandler := metricsHook.WriteHandler(func(filename string, wt io.WriterTo) error {
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
    })

    // Create a new TFTP server with our handlers
    s := tftp.NewServer(readHandler, writeHandler)

	// Wrap original hook kwih mertrics
	//originalHook := &logHook{}
	s.SetHook(originalHook)
    
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

func updateDB(machine Machine)(bool, error){
    connStr := "postgres://nnag:password@10.70.1.1:54321/dhcpdb?sslmode=disable"

    // Initialize DB handler
    dbHandler, err := NewDBHandler(connStr)
    if err != nil {
        return false, fmt.Errorf("failed to connect to database: %v", err)
    }
    defer dbHandler.Close()

    ctx := context.Background()

    lease := Lease{
        IPAddress:        machine.IP,
        MACAddress:       machine.MAC,
        Hostname:         machine.Hostname,
        LeaseStart:       time.Now(),
        LeaseEnd:         time.Now().Add(12 * time.Hour),
        BindingState:     "inactive",
        LastTransaction:  time.Now(),
        NextBindingState: "expired",
        BootfileURL:      "/pxelinux.0",
        TFTPServer:       "192.168.8.103" ,
        IPPoolID:         1,
    }

    // ID           int      `json:"id"`
    // Hostname     string   `json:"hostname"`
    // IP           string   `json:"ip"`
    // MAC          string   `json:"mac"`
    // Role         string   `json:"role"`
    // OSType       string   `json:"os_type"`
    // Status       string   `json:"status"`
    // Health       string   `json:"health"`
    // ClusterID    int      `json:"cluster"`
    // ClusterName  string   `json:"cluster_name,omitempty"`

    leaseStatus, dbErr :=  checkLeases(ctx, "10.80.1.0")

    if dbErr != nil {
        log.Printf("Lease Status error %v", leaseStatus)
        return false, fmt.Errorf("error checking leases: %v", err)
    }

    err = dbHandler.AddLease(ctx, lease)
    if err != nil {
        return false, fmt.Errorf("error adding lease: %v", err)
    }

    return true, nil

}
func invokeDB(cidr string, dhcpMessage *dhcpv4.DHCPv4) (string, string, error) {
    connStr := "postgres://nnag:password@10.70.1.1:54321/dhcpdb?sslmode=disable"

    // Initialize DB handler
    dbHandler, err := NewDBHandler(connStr)
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
    
    lease := Lease{
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

func invokeDBUpdate(connStr string, macAddress string)(string,error){
    // Initialize DB handler
    dbHandler, err := NewDBHandler(connStr)
    if err != nil {
        return "", fmt.Errorf("failed to connect to database: %v", err)
    }
    defer dbHandler.Close()

    ctx := context.Background()

    lease := Lease{
        MACAddress:       macAddress,
        LeaseStart:       time.Now(),
        LeaseEnd:         time.Now().Add(12 * time.Hour),
        BindingState:     "active",
        LastTransaction:  time.Now(),
        NextBindingState: "expired",
        IPPoolID:         1,
    }

    err = dbHandler.UpdateLease(ctx, lease)
    if err != nil {
        return "", fmt.Errorf("error adding lease: %v", err)
    }

    return "", nil
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


// Kubernetes related 
func queryNodeMetrics(ctx context.Context, fetcher *MetricsFetcher, promNamespace, promService string) error {
	fmt.Println("=== Node CPU Usage ===")
	cpuResponse, err := fetcher.QueryNodeCPU(ctx, promNamespace, promService)
	if err != nil {
		return fmt.Errorf("failed to query node CPU: %w", err)
	}

	for _, result := range cpuResponse.Data.Result {
		instance := result.Metric["instance"]
		if len(result.Value) >= 2 {
			fmt.Printf("Node: %s, CPU Usage: %.2f%%\n", instance, result.Value[1])
		}
	}

	fmt.Println("=== Node Memory Usage ===")
	memResponse, err := fetcher.QueryNodeMemory(ctx, promNamespace, promService)
	if err != nil {
		return fmt.Errorf("failed to query node memory: %w", err)
	}

	for _, result := range memResponse.Data.Result {
		instance := result.Metric["instance"]
		if len(result.Value) >= 2 {
			fmt.Printf("Node: %s, Memory Usage: %.2f%%\n", instance, result.Value[1])
		}
	}

	return nil
}
