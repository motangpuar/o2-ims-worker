// api/client.go
package api

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
    "sync"
)

type APIClient struct {
    baseURL     string
    workerID    string
    client      *http.Client
    authToken   string
    username    string
    password    string
    mu          sync.RWMutex
}

type AuthResponse struct {
    Access  string `json:"access"`
    Refresh string `json:"refresh"`
}

type WorkerStatus struct {
    Services map[string]ServiceStatus `json:"services"`
    Metrics  map[string]interface{}   `json:"metrics"`
}

type ServiceStatus struct {
    Status    string `json:"status"`
    LastError string `json:"last_error,omitempty"`
}

type LeaseInfo struct {
    IPAddress  string    `json:"ip_address"`
    MACAddress string    `json:"mac_address"`
    Hostname   string    `json:"hostname"`
    LeaseStart time.Time `json:"lease_start"`
    LeaseEnd   time.Time `json:"lease_end"`
    CIDR       string    `json:"cidr"`
    Gateway    string    `json:"gateway"`
}


func NewAPIClient(baseURL, workerID, username, password string) *APIClient {
    return &APIClient{
        baseURL:   baseURL,
        workerID:  workerID,
        username:  username,
        password:  password,
        client:    &http.Client{Timeout: 10 * time.Second},
    }
}

func (c *APIClient) login() error {
    data := map[string]string{
        "username": c.username,
        "password": c.password,
    }

    jsonData, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("failed to marshal login data: %v", err)
    }

    resp, err := c.client.Post(
        c.baseURL+"/api/token/",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return fmt.Errorf("login request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("login failed with status: %d", resp.StatusCode)
    }

    var authResp AuthResponse
    if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
        return fmt.Errorf("failed to decode auth response: %v", err)
    }

    c.mu.Lock()
    c.authToken = authResp.Access
    c.mu.Unlock()

    return nil
}

func (c *APIClient) getAuthHeader() string {
    c.mu.RLock()
    token := c.authToken
    c.mu.RUnlock()
    return "Bearer " + token
}

func (c *APIClient) doRequest(method, endpoint string, data interface{}) error {
    // Try request with current token
    err := c.doRequestWithAuth(method, endpoint, data)
    if err != nil && (err.Error() == "unauthorized" || err.Error() == "token expired") {
        // Try to login again
        if err := c.login(); err != nil {
            return fmt.Errorf("login retry failed: %v", err)
        }
        // Retry request with new token
        return c.doRequestWithAuth(method, endpoint, data)
    }
    return err
}

func (c *APIClient) doRequestWithAuth(method, endpoint string, data interface{}) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("failed to marshal data: %v", err)
    }

    req, err := http.NewRequest(
        method,
        c.baseURL+endpoint,
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", c.getAuthHeader())

    resp, err := c.client.Do(req)
    if err != nil {
        return fmt.Errorf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return fmt.Errorf("unauthorized")
    }

    if resp.StatusCode != http.StatusOK {
        var errorResp map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
            return fmt.Errorf("request failed with status %d", resp.StatusCode)
        }
        return fmt.Errorf("request failed: %v", errorResp)
    }

    return nil
}

func (c *APIClient) Register(ipAddress string) error {
    // First ensure we're logged in
    if err := c.login(); err != nil {
        return fmt.Errorf("initial login failed: %v", err)
    }

    data := map[string]string{
        "worker_id": c.workerID,
        "ip_address": ipAddress,
    }

    return c.doRequest("POST", "/api/ims-worker/register/", data)
}

func (c *APIClient) SendHeartbeat(status WorkerStatus) error {
    data := map[string]interface{}{
        "worker_id": c.workerID,
        "services": status.Services,
        "metrics": status.Metrics,
    }

    return c.doRequest("POST", "/api/ims-worker/heartbeat/", data)
}

func (c *APIClient) ReportLease(lease LeaseInfo) error {
    data := map[string]interface{}{
        "worker_id": c.workerID,
        "lease": lease,
    }

    return c.doRequest("POST", "/api/ims-worker/report_lease/", data)
}

func (c *APIClient) ReportError(message string, details map[string]interface{}) error {
    data := map[string]interface{}{
        "worker_id": c.workerID,
        "error": map[string]interface{}{
            "message": message,
            "details": details,
        },
    }

    return c.doRequest("POST", "/api/ims-worker/report_error/", data)
}

// GET Functions
type Machine struct {
    ID           int      `json:"id"`
    Hostname     string   `json:"hostname"`
    IP           string   `json:"ip"`
    MAC          string   `json:"mac"`
    Role         string   `json:"role"`
    OSType       string   `json:"os_type"`
    Status       string   `json:"status"`
    Health       string   `json:"health"`
    ClusterID    int      `json:"cluster"`
    ClusterName  string   `json:"cluster_name,omitempty"`
}

type Cluster struct {
    ID          int       `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Status      string    `json:"status"`
    Health      string    `json:"health"`
    Machines    []Machine `json:"machines"`
}

// Add these methods to your APIClient
func (c *APIClient) GetMachines() ([]Machine, error) {
    // First ensure we're logged in
    if err := c.login(); err != nil {
        return nil, fmt.Errorf("initial login failed: %v", err)
    }

    req, err := http.NewRequest("GET", c.baseURL+"/api/machines/", nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", c.getAuthHeader())

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        // Try to login again
        if err := c.login(); err != nil {
            return nil, fmt.Errorf("login retry failed: %v", err)
        }
        // Retry the request
        return c.GetMachines()
    }

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
    }

    var machines []Machine
    if err := json.NewDecoder(resp.Body).Decode(&machines); err != nil {
        return nil, fmt.Errorf("failed to decode response: %v", err)
    }

    return machines, nil
}

func (c *APIClient) GetCluster(clusterID int) (*Cluster, error) {
    // First ensure we're logged in
    if err := c.login(); err != nil {
        return nil, fmt.Errorf("initial login failed: %v", err)
    }

    url := fmt.Sprintf("%s/api/clusters/%d/", c.baseURL, clusterID)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", c.getAuthHeader())

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        // Try to login again
        if err := c.login(); err != nil {
            return nil, fmt.Errorf("login retry failed: %v", err)
        }
        // Retry the request
        return c.GetCluster(clusterID)
    }

    if resp.StatusCode == http.StatusNotFound {
        return nil, fmt.Errorf("cluster not found: %d", clusterID)
    }

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
    }

    var cluster Cluster
    if err := json.NewDecoder(resp.Body).Decode(&cluster); err != nil {
        return nil, fmt.Errorf("failed to decode response: %v", err)
    }

    return &cluster, nil
}

func (c *APIClient) GetMachinesByCluster(clusterID int) ([]Machine, error) {
    cluster, err := c.GetCluster(clusterID)
    if err != nil {
        return nil, err
    }
    return cluster.Machines, nil
}

// Example usage:
/*
func main() {
    client := NewAPIClient(
        "http://localhost:8000",
        "worker-1",
        "username",
        "password",
    )
    
    // Get all machines
    machines, err := client.GetMachines()
    if err != nil {
        log.Printf("Failed to get machines: %v", err)
        return
    }
    
    for _, machine := range machines {
        fmt.Printf("Machine: %s (IP: %s, Status: %s)\n", 
            machine.Hostname, machine.IP, machine.Status)
    }

    // Get specific cluster
    cluster, err := client.GetCluster(1)
    if err != nil {
        log.Printf("Failed to get cluster: %v", err)
        return
    }
    
    fmt.Printf("Cluster: %s (%d machines)\n", 
        cluster.Name, len(cluster.Machines))
}
*/
