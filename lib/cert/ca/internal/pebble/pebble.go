// Package pebble starts a local Pebble + challtestsrv pair for ACME
// adapter integration tests.
//
// Pebble (https://github.com/letsencrypt/pebble) is Let's Encrypt's
// RFC 8555-compliant ACME test server. Together with its companion
// pebble-challtestsrv it forms a fully self-contained ACME ↔ DNS-01
// loop that runs entirely on the test host — no external network,
// no real DNS, no real CA quota.
//
// Usage from an adapter integration test:
//
//	h, err := pebble.Start(t)
//	if errors.Is(err, pebble.ErrSkip) {
//	    t.Skip(err.Error())
//	}
//	require.NoError(t, err)
//	defer h.Close()
//
//	c := letsencrypt.New(letsencrypt.Config{DirectoryURL: h.DirectoryURL})
//	res, err := c.RequestCertificate(ctx, ca.CertificateRequest{
//	    DNS: h.Solver(2 * time.Minute),
//	    ...
//	})
//
// The harness has two execution modes:
//
//   - Env-var mode. If PEBBLE_DIRECTORY_URL is set we trust that an
//     external Pebble + challtestsrv pair is already up (e.g. CI brought
//     them up via docker-compose). DirectoryURL / DNSServer / the
//     challtestsrv management endpoint are read from env vars.
//
//   - Docker mode. Otherwise we shell out to `docker run` and bring up
//     both containers ourselves, capture their endpoints, write the
//     Pebble CA root to a tmp file, and `os.Setenv` LEGO_CA_CERTIFICATES
//     so lego trusts Pebble's self-signed HTTPS. Close kills both
//     containers.
//
// Either mode returns (nil, ErrSkip) when the host cannot run the
// integration: missing docker, daemon down, non-Linux host, etc.
// Test bodies should `t.Skip` on ErrSkip rather than fail; the goal
// is "runs on CI + dev box, never fails the unit suite".
package pebble

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/cert/ca"
)

// ErrSkip is returned by Start when the host environment cannot run a
// Pebble integration test (no docker, daemon not running, non-Linux
// host, etc.). Callers should `t.Skip(err.Error())`.
var ErrSkip = errors.New("pebble: integration prerequisites missing")

// Default endpoints used when the harness boots its own containers in
// --network host mode. These match the upstream Pebble defaults.
const (
	defaultPebbleHTTPSPort   = "14000"
	defaultPebbleMgmtPort    = "15000"
	defaultChallSrvDNSPort   = "8053"
	defaultChallSrvHTTPPort  = "8055"
	defaultChallSrvMgmtHost  = "127.0.0.1"
	defaultPebbleHost        = "127.0.0.1"
	pebbleImage              = "letsencrypt/pebble:latest"
	pebbleChalltestsrvImage  = "letsencrypt/pebble-challtestsrv:latest"
	pebbleCAFetchPathRootKey = "/roots/0"
)

// Timeouts that tests in this package override to keep the unit suite
// fast. Package-private — production callers go through Start, which
// uses these values.
var (
	healthcheckTimeout      = 30 * time.Second
	healthcheckPollInterval = 500 * time.Millisecond
)

// Harness is a running Pebble + challtestsrv pair.
type Harness struct {
	// DirectoryURL is the Pebble ACME directory URL ("https://host:14000/dir").
	DirectoryURL string

	// DNSServer is the host:port challtestsrv is listening on for DNS
	// queries from Pebble's VA (default "127.0.0.1:8053"). Exposed for
	// debugging; ca clients don't need it directly.
	DNSServer string

	// chalMgmtURL is the http://host:port root of challtestsrv's HTTP
	// management API (not the DNS port). Used by SetTXT / ClearTXT.
	chalMgmtURL string

	// caRootFile is the on-disk path to Pebble's self-signed CA root
	// PEM. Lego trusts it via the LEGO_CA_CERTIFICATES env var which
	// we set in Start and unset in Close.
	caRootFile string

	// previousLegoCA holds the value of LEGO_CA_CERTIFICATES at Start
	// time so Close can restore it.
	previousLegoCA    string
	previousLegoCASet bool

	// containers are the docker container IDs we started; empty in
	// env-var mode. Close runs `docker kill` for each.
	containers []string

	// httpClient is reused for SetTXT / ClearTXT and for healthchecks.
	httpClient *http.Client

	closeOnce sync.Once
}

// Start brings up a Pebble harness, or returns (nil, ErrSkip) when the
// host environment can't run integration tests.
//
// The returned Harness MUST be Close()'d to release docker containers
// and restore LEGO_CA_CERTIFICATES. Tests typically `defer h.Close()`.
func Start(t *testing.T) (*Harness, error) {
	t.Helper()

	// Mode A: external Pebble (e.g. CI brought it up via docker-compose).
	if dir := os.Getenv("PEBBLE_DIRECTORY_URL"); dir != "" {
		return startFromEnv(dir)
	}

	// Mode B: spin up our own docker containers. Restricted to Linux
	// because --network host (the simplest cross-container DNS setup)
	// only works there. Mac / Windows users can run the env-var mode
	// against docker-compose.
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("%w: docker mode only supported on linux; set PEBBLE_DIRECTORY_URL for env-var mode", ErrSkip)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("%w: docker binary not found in PATH", ErrSkip)
	}
	if err := dockerDaemonReachable(); err != nil {
		return nil, fmt.Errorf("%w: docker daemon not reachable: %v", ErrSkip, err)
	}
	return startDocker(t)
}

// startFromEnv wires a Harness to an externally-managed Pebble.
//
// Env vars consulted:
//
//	PEBBLE_DIRECTORY_URL          required, e.g. https://localhost:14000/dir
//	PEBBLE_CHALLTESTSRV_URL       optional, default http://127.0.0.1:8055
//	PEBBLE_DNS_SERVER             optional, default 127.0.0.1:8053
//	LEGO_CA_CERTIFICATES          optional, must already point at the Pebble CA root
func startFromEnv(dir string) (*Harness, error) {
	chalMgmt := os.Getenv("PEBBLE_CHALLTESTSRV_URL")
	if chalMgmt == "" {
		chalMgmt = "http://" + defaultChallSrvMgmtHost + ":" + defaultChallSrvHTTPPort
	}
	dnsAddr := os.Getenv("PEBBLE_DNS_SERVER")
	if dnsAddr == "" {
		dnsAddr = defaultChallSrvMgmtHost + ":" + defaultChallSrvDNSPort
	}

	h := &Harness{
		DirectoryURL: dir,
		DNSServer:    dnsAddr,
		chalMgmtURL:  strings.TrimRight(chalMgmt, "/"),
		httpClient:   newInsecureHTTPClient(),
	}
	// Healthcheck both endpoints; if either is down the run will fail
	// noisily anyway, surface as ErrSkip here for clean test output.
	if err := h.waitReady(); err != nil {
		return nil, fmt.Errorf("%w: external pebble not ready: %v", ErrSkip, err)
	}
	return h, nil
}

// startDocker brings up Pebble + challtestsrv via `docker run` and
// returns a Harness pointing at them.
//
// We use --network host (Linux-only) so:
//   - Pebble's VA can reach challtestsrv at 127.0.0.1:8053 for DNS-01
//     resolution (we pass -dnsserver 127.0.0.1:8053)
//   - the test process can reach Pebble at https://127.0.0.1:14000/dir
//     without any port-publishing dance
//
// PEBBLE_VA_NOSLEEP=1 disables Pebble's deliberate 5s reorder delay
// to keep test runs ≤ ~15s. PEBBLE_WFE_NONCEREJECT=0 keeps the nonce
// invalidation rate at 0% so transient nonce errors don't show up.
func startDocker(t *testing.T) (*Harness, error) {
	t.Helper()

	// Pre-flight: try to pull the images if missing. We don't fail
	// hard here — if pull fails the subsequent `docker run` will
	// fail with a clearer error.
	for _, img := range []string{pebbleImage, pebbleChalltestsrvImage} {
		_ = exec.Command("docker", "pull", img).Run()
	}

	// Start challtestsrv first; Pebble's -dnsserver flag points at it
	// during startup so the order matters.
	challArgs := []string{
		"run", "-d", "--rm",
		"--network", "host",
		pebbleChalltestsrvImage,
		"pebble-challtestsrv",
		"-defaultIPv4", "127.0.0.1",
		"-defaultIPv6", "",
		"-dns01", ":" + defaultChallSrvDNSPort,
		"-management", ":" + defaultChallSrvHTTPPort,
		// Disable HTTP-01 / TLS-ALPN-01 / DoH ports we don't use,
		// they collide with anything else listening on those ports.
		"-http01", "",
		"-https01", "",
		"-tlsalpn01", "",
		"-doh", "",
	}
	challID, err := dockerRunBackground(challArgs)
	if err != nil {
		return nil, fmt.Errorf("%w: start challtestsrv: %v", ErrSkip, err)
	}

	pebbleArgs := []string{
		"run", "-d", "--rm",
		"--network", "host",
		"-e", "PEBBLE_VA_NOSLEEP=1",
		"-e", "PEBBLE_WFE_NONCEREJECT=0",
		pebbleImage,
		"pebble",
		"-dnsserver", defaultChallSrvMgmtHost + ":" + defaultChallSrvDNSPort,
	}
	pebbleID, err := dockerRunBackground(pebbleArgs)
	if err != nil {
		_ = dockerKill(challID)
		return nil, fmt.Errorf("%w: start pebble: %v", ErrSkip, err)
	}

	h := &Harness{
		DirectoryURL: "https://" + defaultPebbleHost + ":" + defaultPebbleHTTPSPort + "/dir",
		DNSServer:    defaultChallSrvMgmtHost + ":" + defaultChallSrvDNSPort,
		chalMgmtURL:  "http://" + defaultChallSrvMgmtHost + ":" + defaultChallSrvHTTPPort,
		containers:   []string{pebbleID, challID},
		httpClient:   newInsecureHTTPClient(),
	}

	if err := h.waitReady(); err != nil {
		h.Close()
		return nil, fmt.Errorf("%w: pebble healthcheck failed: %v", ErrSkip, err)
	}

	// Fetch Pebble's CA root and tell lego to trust it. Without this
	// every newOrder POST fails TLS verification.
	caPEM, err := h.fetchPebbleRoot()
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("%w: fetch pebble root: %v", ErrSkip, err)
	}
	f, err := os.CreateTemp("", "pebble-root-*.pem")
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("%w: write pebble root: %v", ErrSkip, err)
	}
	if _, err := f.Write(caPEM); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		h.Close()
		return nil, fmt.Errorf("%w: write pebble root: %v", ErrSkip, err)
	}
	_ = f.Close()
	h.caRootFile = f.Name()

	h.previousLegoCA, h.previousLegoCASet = os.LookupEnv("LEGO_CA_CERTIFICATES")
	if err := os.Setenv("LEGO_CA_CERTIFICATES", h.caRootFile); err != nil {
		h.Close()
		return nil, fmt.Errorf("%w: setenv LEGO_CA_CERTIFICATES: %v", ErrSkip, err)
	}

	return h, nil
}

// SetTXT writes a TXT record into challtestsrv. The fqdn must be the
// fully-qualified _acme-challenge.<domain>. form (trailing dot
// optional; challtestsrv accepts both).
func (h *Harness) SetTXT(ctx context.Context, fqdn, value string) error {
	if h == nil {
		return errors.New("pebble: nil harness")
	}
	body, _ := json.Marshal(map[string]string{
		"host":  ensureTrailingDot(fqdn),
		"value": value,
	})
	return h.postJSON(ctx, "/set-txt", body)
}

// ClearTXT removes the TXT record previously set by SetTXT.
func (h *Harness) ClearTXT(ctx context.Context, fqdn string) error {
	if h == nil {
		return errors.New("pebble: nil harness")
	}
	body, _ := json.Marshal(map[string]string{
		"host": ensureTrailingDot(fqdn),
	})
	return h.postJSON(ctx, "/clear-txt", body)
}

// Solver returns a ca.DnsSolver that pushes records to challtestsrv.
// The timeout is propagated as the solver's reported Timeout() for the
// lego provider's DNS-propagation wait; challtestsrv answers
// immediately so this is mostly a safety net.
func (h *Harness) Solver(timeout time.Duration) ca.DnsSolver {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &challtestsrvSolver{h: h, timeout: timeout}
}

// Close releases harness resources: docker containers (in docker mode),
// restores LEGO_CA_CERTIFICATES, removes the tmp CA root file. Safe to
// call more than once.
func (h *Harness) Close() {
	if h == nil {
		return
	}
	h.closeOnce.Do(func() {
		for _, id := range h.containers {
			_ = dockerKill(id)
		}
		if h.caRootFile != "" {
			_ = os.Remove(h.caRootFile)
		}
		if h.previousLegoCASet {
			_ = os.Setenv("LEGO_CA_CERTIFICATES", h.previousLegoCA)
		} else if len(h.containers) > 0 {
			// Only unset when we set it; in env-var mode the caller
			// owns LEGO_CA_CERTIFICATES.
			_ = os.Unsetenv("LEGO_CA_CERTIFICATES")
		}
	})
}

// waitReady polls Pebble's directory endpoint and challtestsrv's
// management endpoint until both respond 200, up to healthcheckTimeout.
func (h *Harness) waitReady() error {
	deadline := time.Now().Add(healthcheckTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		dirOK := h.pingOK(h.DirectoryURL, http.StatusOK)
		mgmtOK := h.pingOK(h.chalMgmtURL+"/clear-txt", -1)
		if dirOK && mgmtOK {
			return nil
		}
		lastErr = fmt.Errorf("dirOK=%v mgmtOK=%v", dirOK, mgmtOK)
		time.Sleep(healthcheckPollInterval)
	}
	if lastErr == nil {
		lastErr = errors.New("timeout")
	}
	return lastErr
}

// pingOK does a single HEAD-ish probe and reports whether it succeeded.
// For challtestsrv we POST an empty body to /clear-txt; it returns 400
// (missing host) which is fine — what we want is "TCP up + HTTP layer
// responds", not "endpoint accepts this request".
func (h *Harness) pingOK(url string, wantStatus int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var req *http.Request
	var err error
	if strings.Contains(url, "/clear-txt") {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	}
	if err != nil {
		return false
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if wantStatus < 0 {
		return resp.StatusCode > 0
	}
	return resp.StatusCode == wantStatus
}

// fetchPebbleRoot pulls https://<mgmt>:15000/roots/0, Pebble's
// auto-generated intermediate-signing CA root.
func (h *Harness) fetchPebbleRoot() ([]byte, error) {
	url := "https://" + defaultPebbleHost + ":" + defaultPebbleMgmtPort + pebbleCAFetchPathRootKey
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	buf := &bytes.Buffer{}
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// postJSON POSTs to the challtestsrv management endpoint. challtestsrv
// returns 200 + empty body on success.
func (h *Harness) postJSON(ctx context.Context, path string, body []byte) error {
	url := h.chalMgmtURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("challtestsrv post %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("challtestsrv %s: status %d", path, resp.StatusCode)
	}
	return nil
}

// newInsecureHTTPClient returns an http.Client that skips TLS
// verification (used internally to talk to Pebble's self-signed
// management port for healthchecks and root fetching; the real lego
// client uses LEGO_CA_CERTIFICATES, not this client).
func newInsecureHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // internal test harness
		},
	}
}

// dockerDaemonReachable performs `docker info` with a 3s timeout and
// returns the error if the daemon is unreachable.
func dockerDaemonReachable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%v: %s", err, msg)
		}
		return err
	}
	return nil
}

// dockerRunBackground runs `docker run -d ... image cmd...` and returns
// the container ID on success.
func dockerRunBackground(args []string) (string, error) {
	cmd := exec.Command("docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker run failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	id := strings.TrimSpace(stdout.String())
	if id == "" {
		return "", errors.New("docker run returned empty container id")
	}
	return id, nil
}

// dockerKill stops a container by ID. Errors are returned for the
// caller to log but should not fail tests.
func dockerKill(id string) error {
	if id == "" {
		return nil
	}
	cmd := exec.Command("docker", "kill", id)
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run()
}

// ensureTrailingDot normalises an fqdn to its absolute form so
// challtestsrv's DNS resolver matches Pebble's lookup.
func ensureTrailingDot(fqdn string) string {
	if strings.HasSuffix(fqdn, ".") {
		return fqdn
	}
	return fqdn + "."
}

// challtestsrvSolver adapts the harness into a ca.DnsSolver so it can be
// passed straight into ca.CertificateRequest.DNS.
type challtestsrvSolver struct {
	h       *Harness
	timeout time.Duration
}

func (s *challtestsrvSolver) Present(ctx context.Context, fqdn, value string) error {
	return s.h.SetTXT(ctx, fqdn, value)
}

func (s *challtestsrvSolver) CleanUp(ctx context.Context, fqdn, _ string) error {
	return s.h.ClearTXT(ctx, fqdn)
}

func (s *challtestsrvSolver) Timeout() time.Duration { return s.timeout }
