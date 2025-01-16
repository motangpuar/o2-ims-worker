// api/client.go
package api

import (
    "bytes"
    "encoding/json"
    "fmt"
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
