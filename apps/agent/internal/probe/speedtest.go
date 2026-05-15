package probe

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// defaultDownloadSize is 10 MiB — large enough to saturate typical connections
	// without being so large that it causes timeouts on slow links.
	defaultDownloadSize int64 = 10 * 1024 * 1024

	// uploadSize is 1 MiB — smaller than download to keep the test symmetric on
	// asymmetric connections without exhausting upload quota on metered links.
	uploadSize int64 = 1 * 1024 * 1024
)

// Execute measures download and upload bandwidth by transferring fixed-size HTTP payloads.
func (p *SpeedtestProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	// Normalise: bare hostnames become https:// URLs so http.NewRequest accepts them.
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "https://" + target
	}

	data := map[string]any{
		"server":         target,
		"download_mbps":  float64(0),
		"upload_mbps":    float64(0),
		"download_bytes": int64(0),
		"upload_bytes":   int64(0),
		"latency_ms":     int64(0),
	}

	client := &http.Client{Timeout: timeout}

	// --- Download ---
	ctx, cancelDL := context.WithTimeout(context.Background(), timeout)
	defer cancelDL()

	downloadSize := getInt64Option(options, "download_bytes", defaultDownloadSize)

	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return &Result{
			Type:       TaskSpeedtest,
			Target:     target,
			Success:    false,
			Error:      fmt.Sprintf("create download request: %v", err),
			Data:       data,
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  start,
		}
	}
	dlReq.Header.Set("Range", fmt.Sprintf("bytes=0-%d", downloadSize-1))

	dlStart := time.Now()
	dlResp, err := client.Do(dlReq)
	if err != nil {
		return &Result{
			Type:       TaskSpeedtest,
			Target:     target,
			Success:    false,
			Error:      fmt.Sprintf("download failed: %v", err),
			Data:       data,
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  start,
		}
	}
	defer dlResp.Body.Close()

	// Record first-byte latency before reading the full body.
	latencyMs := time.Since(dlStart).Milliseconds()

	bytesRead, err := io.Copy(io.Discard, dlResp.Body)
	if err != nil {
		return &Result{
			Type:       TaskSpeedtest,
			Target:     target,
			Success:    false,
			Error:      fmt.Sprintf("read download body: %v", err),
			Data:       data,
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  start,
		}
	}
	dlElapsed := time.Since(dlStart)

	var downloadMbps float64
	if dlElapsed > 0 && bytesRead > 0 {
		downloadMbps = float64(bytesRead) * 8 / dlElapsed.Seconds() / 1e6
	}

	data["download_mbps"] = downloadMbps
	data["download_bytes"] = bytesRead
	data["latency_ms"] = latencyMs

	// --- Upload ---
	ctx2, cancelUL := context.WithTimeout(context.Background(), timeout)
	defer cancelUL()

	ulBody := io.LimitReader(rand.Reader, uploadSize)
	ulReq, err := http.NewRequestWithContext(ctx2, http.MethodPost, target, ulBody)
	if err != nil {
		// Upload failure is non-fatal; report what we have so far.
		data["upload_error"] = fmt.Sprintf("create upload request: %v", err)
	} else {
		ulReq.ContentLength = uploadSize
		ulReq.Header.Set("Content-Type", "application/octet-stream")

		ulStart := time.Now()
		ulResp, ulErr := client.Do(ulReq)
		if ulErr != nil {
			data["upload_error"] = fmt.Sprintf("upload failed: %v", ulErr)
		} else {
			ulResp.Body.Close()
			// Only count bandwidth if the server accepted the payload (2xx).
			if ulResp.StatusCode >= 200 && ulResp.StatusCode < 300 {
				ulElapsed := time.Since(ulStart)
				if ulElapsed > 0 {
					data["upload_mbps"] = float64(uploadSize) * 8 / ulElapsed.Seconds() / 1e6
				}
				data["upload_bytes"] = uploadSize
			} else {
				data["upload_error"] = fmt.Sprintf("upload rejected: HTTP %d", ulResp.StatusCode)
			}
		}
	}

	return &Result{
		Type:       TaskSpeedtest,
		Target:     target,
		Success:    true,
		Data:       data,
		DurationMs: time.Since(start).Milliseconds(),
		Timestamp:  start,
	}
}

// getInt64Option retrieves an int64 option with a default fallback.
func getInt64Option(options map[string]any, key string, defaultValue int64) int64 {
	if v, ok := options[key]; ok {
		switch n := v.(type) {
		case int64:
			return n
		case float64:
			return int64(n)
		case int:
			return int64(n)
		}
	}
	return defaultValue
}
