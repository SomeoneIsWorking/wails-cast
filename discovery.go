package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/koron/go-ssdp"
)

// Device represents a cast device
type Device struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Address     string `json:"address"`
	ManufactURL string `json:"manufacturerUrl"`
}

type DeviceDiscovery struct{}

func NewDeviceDiscovery() *DeviceDiscovery {
	return &DeviceDiscovery{}
}

// Discover finds available cast devices on the network
func (dd *DeviceDiscovery) Discover() ([]Device, error) {
	devices := []Device{}
	deviceMap := make(map[string]Device)

	results, err := ssdp.Search(ssdp.All, 3, "")
	if err != nil {
		return devices, fmt.Errorf("SSDP search failed: %w", err)
	}

	for _, result := range results {
		if strings.Contains(result.Type, "upnp:rootdevice") ||
			strings.Contains(result.Type, "urn:dial-multiscreen-org") {
			device := Device{
				Name:    dd.extractName(result.USN),
				Type:    "Chromecast/DLNA",
				URL:     result.Location,
				Address: dd.extractAddress(result.Location),
			}
			deviceMap[device.URL] = device
		}
	}

	for _, device := range deviceMap {
		devices = append(devices, device)
	}

	return devices, nil
}

// extractName extracts device name from USN
func (dd *DeviceDiscovery) extractName(usn string) string {
	parts := strings.Split(usn, "::")
	if len(parts) > 0 {
		return parts[0]
	}
	return "Unknown Device"
}

// extractAddress extracts IP address from URL
func (dd *DeviceDiscovery) extractAddress(urlStr string) string {
	parts := strings.Split(urlStr, "//")
	if len(parts) > 1 {
		return strings.Split(parts[1], ":")[0]
	}
	return ""
}

// GetLocalIP returns the local IP address
func (dd *DeviceDiscovery) GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
