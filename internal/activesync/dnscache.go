package activesync

// Resilient DNS for mobile: phones doze, VPNs (ZeroTier) sleep, and wifi
// roams — all of which make hostname lookups fail intermittently with
// "no such host", killing a sync mid-flight even though the server itself
// is reachable. Strategy: remember the last IP that worked for a host and
// fall back to it whenever a live lookup fails. The IP is persisted to disk
// (SetDNSCachePath) so it survives app restarts. TLS still uses the URL's
// hostname; BOI's EAS transport does not do cert verification, and even if
// it did, ServerName is taken from the URL, not the dial address.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	dnsMu        sync.Mutex
	dnsCache     = map[string]string{}
	dnsCachePath string
)

// SetDNSCachePath points the resolver cache at a JSON file. Call once at
// startup with a writable app-private path.
func SetDNSCachePath(p string) {
	dnsMu.Lock()
	dnsCachePath = p
	data, err := os.ReadFile(p)
	if err == nil {
		var m map[string]string
		if json.Unmarshal(data, &m) == nil {
			dnsCache = m
		}
	}
	dnsMu.Unlock()
}

func dnsStore(host, ip string) {
	dnsMu.Lock()
	dnsCache[host] = ip
	path := dnsCachePath
	m := make(map[string]string, len(dnsCache))
	for k, v := range dnsCache {
		m[k] = v
	}
	dnsMu.Unlock()

	if path != "" {
		if data, err := json.Marshal(m); err == nil {
			_ = os.MkdirAll(filepath.Dir(path), 0700)
			_ = os.WriteFile(path, data, 0600)
		}
	}
}

func dnsCached(host string) string {
	dnsMu.Lock()
	defer dnsMu.Unlock()
	return dnsCache[host]
}

// resolveHost returns an IP for host: live lookup first (4s cap so a dead
// resolver fails fast), last-known-good IP as fallback.
func resolveHost(host string) (string, error) {
	if host == "" {
		return "", fmt.Errorf("empty host")
	}
	if net.ParseIP(host) != nil {
		return host, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err == nil && len(ips) > 0 {
		dnsStore(host, ips[0])
		return ips[0], nil
	}
	if ip := dnsCached(host); ip != "" {
		return ip, nil
	}
	return "", err
}

// dialContext is the http.Transport DialContext: resolve via resolveHost,
// then dial the IP directly.
func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip, err := resolveHost(host)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	return d.DialContext(ctx, network, net.JoinHostPort(ip, port))
}
