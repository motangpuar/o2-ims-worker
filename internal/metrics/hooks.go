package metrics

import (
	"io"
	"path/filepath"
	"strings"
	"time"
	"os"
	"fmt"

)

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
