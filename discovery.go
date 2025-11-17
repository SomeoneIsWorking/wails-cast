package main

import (
	"context"
	"fmt"
	"net"
	"time"

	castdns "github.com/vishen/go-chromecast/dns"
)

// Device represents a cast device
type Device struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Address string `json:"address"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	UUID    string `json:"uuid"`
}

type DeviceDiscovery struct{}

func NewDeviceDiscovery() *DeviceDiscovery {
	return &DeviceDiscovery{}
}

// Discover finds available cast devices using go-chromecast's built-in discovery
func (dd *DeviceDiscovery) Discover() ([]Device, error) {
	logger.Info("Starting device discovery using go-chromecast")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use go-chromecast's cast DNS discovery
	castEntryChan, err := castdns.DiscoverCastDNSEntries(ctx, nil)
	if err != nil {
		logger.Error("Failed to start discovery", "error", err)
		return nil, err
	}

	var devices []Device
	for entry := range castEntryChan {
		device := Device{
			Name:    entry.DeviceName,
			Type:    "Chromecast",
			Host:    entry.AddrV4.String(),
			Port:    entry.Port,
			Address: entry.AddrV4.String(),
			URL:     fmt.Sprintf("http://%s:%d", entry.AddrV4.String(), entry.Port),
			UUID:    entry.UUID,
		}
		devices = append(devices, device)
		logger.Info("Found device", "name", device.Name, "host", device.Host, "port", device.Port, "uuid", device.UUID)
	}

	logger.Info("Discovery complete", "count", len(devices))
	return devices, nil
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
