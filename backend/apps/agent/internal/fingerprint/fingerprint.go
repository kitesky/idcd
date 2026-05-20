// Package fingerprint collects stable hardware/OS identifiers for a node.
// Used in heartbeats so the gateway can detect unexpected host changes.
package fingerprint

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
)

// Fingerprint holds the stable identifiers of the host machine.
type Fingerprint struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kernel   string `json:"kernel"`
	MAC      string `json:"mac"`       // first non-loopback hardware MAC
	CPUModel string `json:"cpu_model"` // first physical CPU model string
}

// Collect gathers the current machine fingerprint.
func Collect() (*Fingerprint, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("fingerprint: hostname: %w", err)
	}

	kernel := kernelVersion()
	mac := firstHardwareMAC()
	cpu := cpuModel()

	return &Fingerprint{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Kernel:   kernel,
		MAC:      mac,
		CPUModel: cpu,
	}, nil
}

// Equal reports whether two fingerprints are identical.
func (f *Fingerprint) Equal(other *Fingerprint) bool {
	if f == nil || other == nil {
		return f == other
	}
	return f.Hostname == other.Hostname &&
		f.OS == other.OS &&
		f.Arch == other.Arch &&
		f.Kernel == other.Kernel &&
		f.MAC == other.MAC &&
		f.CPUModel == other.CPUModel
}

// Diff returns a human-readable description of fields that changed.
func (f *Fingerprint) Diff(prev *Fingerprint) string {
	if prev == nil {
		return "no previous fingerprint (first registration)"
	}
	var parts []string
	if f.Hostname != prev.Hostname {
		parts = append(parts, fmt.Sprintf("hostname: %q→%q", prev.Hostname, f.Hostname))
	}
	if f.Kernel != prev.Kernel {
		parts = append(parts, fmt.Sprintf("kernel: %q→%q", prev.Kernel, f.Kernel))
	}
	if f.MAC != prev.MAC {
		parts = append(parts, fmt.Sprintf("mac: %q→%q", prev.MAC, f.MAC))
	}
	if f.CPUModel != prev.CPUModel {
		parts = append(parts, fmt.Sprintf("cpu_model: %q→%q", prev.CPUModel, f.CPUModel))
	}
	if f.Arch != prev.Arch {
		parts = append(parts, fmt.Sprintf("arch: %q→%q", prev.Arch, f.Arch))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ")
}

// firstHardwareMAC returns the MAC address of the first non-loopback,
// non-virtual network interface that has a hardware address.
func firstHardwareMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		// Skip virtual/docker/veth interfaces by name prefix
		name := iface.Name
		if strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "virbr") {
			continue
		}
		return iface.HardwareAddr.String()
	}
	return ""
}

// kernelVersion reads the kernel release from /proc/version on Linux.
// Returns empty string on other platforms or on read failure.
func kernelVersion() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	// "Linux version 5.15.0-... (gcc ...) ..."  → take first three words
	fields := strings.Fields(line)
	if len(fields) >= 3 {
		return fields[2] // kernel release string, e.g. "5.15.0-91-generic"
	}
	return line
}

// cpuModel reads the first "model name" entry from /proc/cpuinfo.
func cpuModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
