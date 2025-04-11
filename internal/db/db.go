package db

import (
	"context"
	"time"
    "fmt"
    "net"

	//"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBHandler struct {
	pool *pgxpool.Pool
}

type Lease struct {
	IPAddress        string
	MACAddress       string
	Hostname         string
	LeaseStart       time.Time
	LeaseEnd         time.Time
	BindingState     string
	LastTransaction  time.Time
	NextBindingState string
	BootfileURL      string
	TFTPServer       string
	IPPoolID         int
}

type IPPool struct {
	ID          int
	CIDR        string
    Gateway     string
	Description string
}

// Initialize the database connection
func NewDBHandler(connStr string) (*DBHandler, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return &DBHandler{pool: pool}, nil
}

// Close the connection pool
func (handler *DBHandler) Close() {
	handler.pool.Close()
}

// Add a new IP pool
func (handler *DBHandler) AddIPPool(ctx context.Context, cidr, gateway, description string) error {
	query := `INSERT INTO ip_pools (cidr, gateway, description) VALUES ($1, $2, $3)`
	_, err := handler.pool.Exec(ctx, query, cidr, gateway, description)
	return err
}

// Get all IP pools
func (handler *DBHandler) GetIPPools(ctx context.Context) ([]IPPool, error) {
	query := `SELECT id, cidr, gateway, description FROM ip_pools`
	rows, err := handler.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pools []IPPool
	for rows.Next() {
		var pool IPPool
		err := rows.Scan(&pool.ID, &pool.CIDR, &pool.Gateway, &pool.Description)
		if err != nil {
			return nil, err
		}
		pools = append(pools, pool)
	}
	return pools, nil
}

// Add a new lease
func (handler *DBHandler) AddLease(ctx context.Context, lease Lease) error {
	query := `
	INSERT INTO leases (
		ip_address, mac_address, hostname, lease_start, lease_end, binding_state,
		last_transaction, next_binding_state, bootfile_url, tftp_server, ip_pool_id
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := handler.pool.Exec(ctx, query,
		lease.IPAddress, lease.MACAddress, lease.Hostname, lease.LeaseStart,
		lease.LeaseEnd, lease.BindingState, lease.LastTransaction,
		lease.NextBindingState, lease.BootfileURL, lease.TFTPServer, lease.IPPoolID)
	return err
}

// Update Lease
func (handler *DBHandler) UpdateLease(ctx context.Context, lease Lease) error {
	//query := `
    //UPDATE leases SET 
	//	ip_address = $1,
    //    mac_address = $2,
    //    hostname = $3,
    //    lease_start = $4,
    //    lease_end = $5,
    //    binding_state = $6,
	//	last_transaction = $7,
    //    next_binding_state = $8,
    //    bootfile_url = $9,
    //    tftp_server = $10,
    //    ip_pool_id = $11
    //WHERE mac_address = $2
	//`
	query := `
    UPDATE leases SET 
        mac_address = $1,
        lease_start = $2,
        lease_end = $3,
        binding_state = $4,
		last_transaction = $5,
        next_binding_state = $6,
        ip_pool_id = $7
    WHERE mac_address = $1
	`
	_, err := handler.pool.Exec(ctx, query,
        lease.MACAddress, lease.LeaseStart, lease.LeaseEnd, lease.BindingState,
        lease.LastTransaction, lease.NextBindingState, lease.IPPoolID)
	return err
}

// Get all leases
func (handler *DBHandler) GetLeases(ctx context.Context) ([]Lease, error) {
	query := `
	SELECT ip_address, mac_address, hostname, lease_start, lease_end, binding_state,
	       last_transaction, next_binding_state, bootfile_url, tftp_server, ip_pool_id
	FROM leases
	`
	rows, err := handler.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leases []Lease
	for rows.Next() {
		var lease Lease
		err := rows.Scan(
			&lease.IPAddress, &lease.MACAddress, &lease.Hostname, &lease.LeaseStart,
			&lease.LeaseEnd, &lease.BindingState, &lease.LastTransaction,
			&lease.NextBindingState, &lease.BootfileURL, &lease.TFTPServer, &lease.IPPoolID)
		if err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	return leases, nil
}

// Function to find an unused IP in an allowed subnet
func (handler *DBHandler) GetAvailableIP(ctx context.Context, cidr string) (string, error) {

    var gatewayIP string
    query := `
        SELECT gateway
        FROM ip_pools
        WHERE cidr = $1
    `
    err := handler.pool.QueryRow(ctx, query, cidr).Scan(&gatewayIP)
    if err != nil {
        return "", fmt.Errorf("Failed to get gateway for CIDR %s: %v", cidr, err)
    }

    // Parse the CIDR
    ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %v", err)
	}
    
	// Convert the base IP to IPv4
	startIP := ip.To4()
	if startIP == nil {
		return "", fmt.Errorf("only IPv4 subnets are supported")
	}

	// Calculate the last valid IP in the subnet
	maskSize, _ := ipNet.Mask.Size()
	totalIPs := 1 << (32 - maskSize) // Total IPs in the subnet
	lastIP := make(net.IP, len(startIP))
	copy(lastIP, startIP)
	for i := 0; i < totalIPs-1; i++ {
		incrementIP(lastIP)
	}

	// Move startIP to the first usable IP (skip network address)
	incrementIP(startIP)

	// Query to get all used IP addresses from the leases table
	query = `
		SELECT ip_address 
		FROM leases
	`
	rows, err := handler.pool.Query(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to query used IPs: %v", err)
	}
	defer rows.Close()

	// Collect all used IPs in a map for quick lookup
	usedIPs := make(map[string]bool)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return "", fmt.Errorf("failed to scan IP address: %v", err)
		}
		usedIPs[ip] = true
	}

	// Iterate over the subnet range to find an unused IP
	for ip := startIP; compareIPs(ip, lastIP) <= 0; incrementIP(ip) {
        if ip.String() == gatewayIP || usedIPs[ip.String()] {
            continue
        }
        return ip.String(), nil
	}

	return "", fmt.Errorf("no available IP in the subnet %s", cidr)
}

// Helper function to increment an IP address
func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// Helper function to compare two IPs
func compareIPs(ip1, ip2 net.IP) int {
	for i := 0; i < len(ip1); i++ {
		if ip1[i] < ip2[i] {
			return -1
		} else if ip1[i] > ip2[i] {
			return 1
		}
	}
	return 0
}

// GetLeaseByMAC retrieves a lease by MAC address
func (handler *DBHandler) GetLeaseByMAC(ctx context.Context, macAddress string) (Lease, error) {
	query := `
		SELECT ip_address, mac_address, hostname, lease_start, lease_end, binding_state,
		       last_transaction, next_binding_state, bootfile_url, tftp_server, ip_pool_id
		FROM leases
		WHERE mac_address = $1
		ORDER BY lease_end DESC
		LIMIT 1
	`
	
	var lease Lease
	err := handler.pool.QueryRow(ctx, query, macAddress).Scan(
		&lease.IPAddress, &lease.MACAddress, &lease.Hostname, &lease.LeaseStart,
		&lease.LeaseEnd, &lease.BindingState, &lease.LastTransaction,
		&lease.NextBindingState, &lease.BootfileURL, &lease.TFTPServer, &lease.IPPoolID)
	
	return lease, err
}
