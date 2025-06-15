package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	"io"

	"strings"
	"os"
	"path/filepath"

	// Internals
    //"git.nnag.me/infidel/boothandler-go/internal/api"
)

// MetricTypes defines all the metric types for PXE-related operations
const (
	// DHCP metrics
	MetricTypeDHCPDiscoverCount     = "pxe.dhcp.discover_count"
	MetricTypeDHCPRequestCount      = "pxe.dhcp.request_count"
	MetricTypeDHCPOfferCount        = "pxe.dhcp.offer_count"
	MetricTypeDHCPAckCount          = "pxe.dhcp.ack_count"
	MetricTypeDHCPNackCount         = "pxe.dhcp.nack_count"
	MetricTypeDHCPLeaseCount        = "pxe.dhcp.lease_count"
	MetricTypeDHCPLeaseUtilization  = "pxe.dhcp.lease_utilization"
	
	// TFTP metrics
	MetricTypeTFTPRequestCount      = "pxe.tftp.request_count"
	MetricTypeTFTPSuccessCount      = "pxe.tftp.success_count"
	MetricTypeTFTPErrorCount        = "pxe.tftp.error_count"
	MetricTypeTFTPTransferRate      = "pxe.tftp.transfer_rate"
	
	// PXE Boot metrics
	MetricTypePXEBootAttempts       = "pxe.boot.attempts"
	MetricTypePXEBootSuccess        = "pxe.boot.success"
	MetricTypePXEBootFailure        = "pxe.boot.failure"
	MetricTypePXEBootTime           = "pxe.boot.time"
	
	// Provisioning metrics
	MetricTypeProvisioningState     = "pxe.provisioning.state"
	MetricTypeProvisioningProgress  = "pxe.provisioning.progress"
	MetricTypeProvisioningDuration  = "pxe.provisioning.duration"
)

// Metric represents a single PXE-related metric
type Metric struct {
	ID           string            `json:"id"`
	MetricType   string            `json:"metric_type"`
	Timestamp    time.Time         `json:"timestamp"`
	Value        float64           `json:"value"`
	ServerMAC    string            `json:"server_mac,omitempty"`
	ServerIP     string            `json:"server_ip,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// MetricsBatch represents a batch of metrics to be reported
type MetricsBatch struct {
	WorkerID string   `json:"worker_id"`
	Metrics  []Metric `json:"metrics"`
}

// Collector collects and reports metrics related to PXE provisioning
type Collector struct {
	workerID       string
	metricsURL     string
	metricsBuffer  []Metric
	mutex          sync.Mutex
	reportInterval time.Duration
	lastReport     time.Time
	httpClient     *http.Client
	username       string
	password       string
	apiClient     *APIClient
}

// NewCollector creates a new metrics collector
func NewCollector(workerID, metricsURL, username, password string, apiClient *APIClient ) *Collector {
	return &Collector{
		workerID:       workerID,
		metricsURL:     metricsURL,
		metricsBuffer:  make([]Metric, 0, 100),
		reportInterval: 30 * time.Second,
		lastReport:     time.Now(),
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		username:       username,
		password:       password,
		apiClient:      apiClient,
	}
}

// SetReportInterval changes the reporting interval
func (c *Collector) SetReportInterval(interval time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.reportInterval = interval
}

// RecordMetric records a new metric
func (c *Collector) RecordMetric(metricType string, value float64, serverMAC, serverIP string, labels map[string]string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Create metric with unique ID based on time
	metric := Metric{
		ID:         fmt.Sprintf("%s-%d", metricType, time.Now().UnixNano()),
		MetricType: metricType,
		Timestamp:  time.Now(),
		Value:      value,
		ServerMAC:  serverMAC,
		ServerIP:   serverIP,
		Labels:     labels,
	}

	// Add to buffer
	c.metricsBuffer = append(c.metricsBuffer, metric)

	// Check if it's time to report metrics
	if len(c.metricsBuffer) >= 50 || time.Since(c.lastReport) >= c.reportInterval {
		go c.ReportMetrics()
	}
}

// RecordDHCPEvent records a DHCP-related event as a metric
func (c *Collector) RecordDHCPEvent(metricType string, macAddress string, ipAddress string, value float64) {
	labels := map[string]string{}
	
	if ipAddress != "" {
		labels["ip_address"] = ipAddress
	}
	
	c.RecordMetric(metricType, value, macAddress, ipAddress, labels)
}

// RecordTFTPRequest records a TFTP request as a metric
func (c *Collector) RecordTFTPRequest(clientIP, filename string, fileSize int64, success bool, errorMessage string, durationMs int) {
	labels := map[string]string{
		"filename": filename,
	}
	
	if errorMessage != "" {
		labels["error"] = errorMessage
	}
	
	if durationMs > 0 {
		labels["duration_ms"] = fmt.Sprintf("%d", durationMs)
	}
	
	metricType := MetricTypeTFTPRequestCount
	if success {
		metricType = MetricTypeTFTPSuccessCount
	} else {
		metricType = MetricTypeTFTPErrorCount
	}
	
	c.RecordMetric(metricType, 1, "", clientIP, labels)
	
	// If successful and duration is provided, record transfer rate
	if success && durationMs > 0 && fileSize > 0 {
		// Calculate transfer rate in KB/s
		transferRate := float64(fileSize) / float64(durationMs) * 1000 / 1024
		c.RecordMetric(MetricTypeTFTPTransferRate, transferRate, "", clientIP, labels)
	}
}

// ReportMetrics sends collected metrics to the metrics API
func (c *Collector) ReportMetrics() {
	c.mutex.Lock()
	if len(c.metricsBuffer) == 0 {
		c.mutex.Unlock()
		return
	}

	// Copy metrics to send and clear buffer
	metricsToSend := make([]Metric, len(c.metricsBuffer))
	copy(metricsToSend, c.metricsBuffer)
	c.metricsBuffer = c.metricsBuffer[:0]
	c.lastReport = time.Now()
	c.mutex.Unlock()

	// Create metrics batch
	batch := MetricsBatch{
		WorkerID: c.workerID,
		Metrics:  metricsToSend,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(batch)
	if err != nil {
		log.Printf("Error marshaling metrics: %v", err)
		return
	}

	log.Printf("JSON Payload \n%s\n", string(jsonData))

	// Create request
	req, err := http.NewRequest("POST", c.metricsURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating metrics request: %v", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	authHeader := c.apiClient.GetAuthHeader()
	//req.SetBasicAuth(c.username, c.password)
	log.Printf("Metric Token: %s\n", authHeader)
    req.Header.Set("Authorization", authHeader)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("Error sending metrics: %v", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("Error response from metrics API: %d", resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Malformed response body\n")
		}
		log.Printf("Response Body: \n%s\n", string(body))
		return
	}

	log.Printf("Successfully reported %d metrics", len(metricsToSend))
}

// GetMetricCounts returns the current count of metrics by type
func (c *Collector) GetMetricCounts() map[string]int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	counts := make(map[string]int)
	for _, metric := range c.metricsBuffer {
		counts[metric.MetricType]++
	}
	
	return counts
}

// TFTPMetricsHook provides TFTP read/write handlers with metrics collection
type TFTPMetricsHook struct {
	collector *Collector
	rootDir   string
}

// NewTFTPMetricsHook creates a new TFTP metrics hook
func NewTFTPMetricsHook(collector *Collector, rootDir string) *TFTPMetricsHook {
	return &TFTPMetricsHook{
		collector: collector,
		rootDir:   rootDir,
	}
}

// ReadHandler returns a TFTP read handler with metrics collection
func (h *TFTPMetricsHook) ReadHandler(originalHandler func(string, io.ReaderFrom) error) func(string, io.ReaderFrom) error {
	return func(filename string, rf io.ReaderFrom) error {
		// Get client information - this may need adjustment based on your tftp library
		var clientIP string
		
		// Attempt to extract remote address if possible
		if remoteAddrProvider, ok := rf.(interface {
			RemoteAddr() string
		}); ok {
			addr := remoteAddrProvider.RemoteAddr()
			clientIP = addr
			// Strip port if present
			if idx := strings.LastIndex(clientIP, ":"); idx > 0 {
				clientIP = clientIP[:idx]
			}
		}

		// Record initial metric before processing
		h.collector.RecordTFTPRequest(clientIP, filename, 0, true, "", 0)

		// Start timer for measuring transfer time
		startTime := time.Now()

		// Call original handler
		err := originalHandler(filename, rf)

		// Calculate duration
		duration := time.Since(startTime)
		durationMs := int(duration / time.Millisecond)

		if err != nil {
			// Record error
			h.collector.RecordTFTPRequest(clientIP, filename, 0, false, err.Error(), durationMs)
			return err
		}

		// Get file size if available
		var fileSize int64
		// Try to check if rf implements Size() method
		if sizer, ok := rf.(interface {
			Size() int64
		}); ok {
			fileSize = sizer.Size()
		} else {
			// Try to get file size from disk as fallback
			if filename[0] == '/' {
				filename = filename[1:]
			}
			fullPath := filepath.Join(h.rootDir, filename)
			if fileInfo, err := os.Stat(fullPath); err == nil {
				fileSize = fileInfo.Size()
			}
		}

		// Record success metric
		h.collector.RecordTFTPRequest(clientIP, filename, fileSize, true, "", durationMs)

		// Check if this is a PXE file
		if isPXEFile(filename) {
			// Record PXE boot attempt
			h.collector.RecordMetric(MetricTypePXEBootAttempts, 1, "", clientIP, map[string]string{
				"filename": filename,
			})
		}

		return nil
	}
}

// WriteHandler returns a TFTP write handler with metrics collection
func (h *TFTPMetricsHook) WriteHandler(originalHandler func(string, io.WriterTo) error) func(string, io.WriterTo) error {
	return func(filename string, wt io.WriterTo) error {
		// Get client information - this may need adjustment based on your tftp library
		var clientIP string
		
		// Attempt to extract remote address if possible
		if remoteAddrProvider, ok := wt.(interface {
			RemoteAddr() string
		}); ok {
			addr := remoteAddrProvider.RemoteAddr()
			clientIP = addr
			// Strip port if present
			if idx := strings.LastIndex(clientIP, ":"); idx > 0 {
				clientIP = clientIP[:idx]
			}
		}

		// Record initial metric before processing
		h.collector.RecordMetric(MetricTypeTFTPRequestCount, 1, "", clientIP, map[string]string{
			"filename": filename,
			"operation": "write",
		})

		// Start timer
		startTime := time.Now()

		// Call original handler
		err := originalHandler(filename, wt)

		// Calculate duration
		duration := time.Since(startTime)
		durationMs := int(duration / time.Millisecond)

		if err != nil {
			// Record error
			h.collector.RecordMetric(MetricTypeTFTPErrorCount, 1, "", clientIP, map[string]string{
				"filename": filename,
				"operation": "write",
				"error": err.Error(),
				"duration_ms": fmt.Sprintf("%d", durationMs),
			})
			return err
		}

		// Record success metric
		h.collector.RecordMetric(MetricTypeTFTPSuccessCount, 1, "", clientIP, map[string]string{
			"filename": filename,
			"operation": "write",
			"duration_ms": fmt.Sprintf("%d", durationMs),
		})

		return nil
	}
}

// MetricsLogHook is a simple struct that can be used with your original logHook
type MetricsLogHook struct {
	collector *Collector
	originalHook interface{}
}

// NewMetricsLogHook creates a new metrics log hook
func NewMetricsLogHook(collector *Collector, originalHook interface{}) *MetricsLogHook {
	return &MetricsLogHook{
		collector: collector,
		originalHook: originalHook,
	}
}

// OnSuccess can be called after a successful transfer
func (h *MetricsLogHook) OnSuccess(clientIP, filename string, bytesTransferred int64, duration time.Duration) {
	// Call original hook OnSuccess if it exists and has the right signature
	if h.originalHook != nil {
		if hook, ok := h.originalHook.(interface {
			OnSuccess(clientIP, filename string, bytesTransferred int64, duration time.Duration)
		}); ok {
			hook.OnSuccess(clientIP, filename, bytesTransferred, duration)
		}
	}

	// Record transfer rate
	if bytesTransferred > 0 && duration > 0 {
		// Calculate KB/s
		transferRate := float64(bytesTransferred) / duration.Seconds() / 1024
		h.collector.RecordMetric(MetricTypeTFTPTransferRate, transferRate, "", clientIP, map[string]string{
			"filename": filename,
		})
	}
}

// OnFailure can be called after a failed transfer
func (h *MetricsLogHook) OnFailure(clientIP, filename string, err error) {
	// Call original hook OnFailure if it exists and has the right signature
	if h.originalHook != nil {
		if hook, ok := h.originalHook.(interface {
			OnFailure(clientIP, filename string, err error)
		}); ok {
			hook.OnFailure(clientIP, filename, err)
		}
	}

	// Record error metric
	h.collector.RecordMetric(MetricTypeTFTPErrorCount, 1, "", clientIP, map[string]string{
		"filename": filename,
		"error": err.Error(),
	})
}

// isPXEFile checks if a filename is related to PXE booting
func isPXEFile(filename string) bool {
	pxePatterns := []string{
		"pxelinux.0",
		"lpxelinux.0",
		"pxelinux.cfg",
		"boot.ipxe",
		"ipxe",
		"bootx64.efi",
		"bootia32.efi",
		"vmlinuz",
		"initrd",
		"kernel",
	}

	lowercaseFilename := strings.ToLower(filename)
	for _, pattern := range pxePatterns {
		if strings.Contains(lowercaseFilename, pattern) {
			return true
		}
	}

	return false
}
