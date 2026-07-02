package castapi

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const service = "_wailscast._tcp"

func Discover(ctx context.Context) ([]CastInstance, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)

	browseCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := resolver.Browse(browseCtx, service, "local.", entries); err != nil {
		return nil, fmt.Errorf("mDNS browse: %w", err)
	}

	localIPs := localIPSet()

	found := make([]CastInstance, 0)
	seen := map[string]bool{}
	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				return found, nil
			}
			if entry == nil {
				continue
			}
			if entryIsSelf(entry, localIPs) {
				continue
			}
			host := ""
			if len(entry.AddrIPv4) > 0 {
				host = entry.AddrIPv4[0].String()
			} else if entry.HostName != "" {
				host = strings.TrimSuffix(entry.HostName, ".")
			}
			if host == "" {
				continue
			}
			key := fmt.Sprintf("%s:%d", host, entry.Port)
			if seen[key] {
				continue
			}
			seen[key] = true
			name := entry.Instance
			if name == "" {
				name = host
			}
			found = append(found, CastInstance{
				Name: name,
				Host: host,
				Port: entry.Port,
				URL:  fmt.Sprintf("http://%s:%d", host, entry.Port),
			})
		case <-browseCtx.Done():
			return found, nil
		}
	}
}

func localIPSet() map[string]bool {
	set := map[string]bool{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return set
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok {
			set[ipNet.IP.String()] = true
		}
	}
	return set
}

func entryIsSelf(entry *zeroconf.ServiceEntry, localIPs map[string]bool) bool {
	for _, ip := range entry.AddrIPv4 {
		if localIPs[ip.String()] {
			return true
		}
	}
	for _, ip := range entry.AddrIPv6 {
		if localIPs[ip.String()] {
			return true
		}
	}
	return false
}
