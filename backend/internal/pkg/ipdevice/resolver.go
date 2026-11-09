// Package ipdevice maps client IP addresses recorded in usage logs to
// human-readable device names, using the local tailscaled LocalAPI plus
// optional manual aliases.
package ipdevice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	SourceTailscale = "tailscale"
	SourceAlias     = "alias"

	defaultCacheTTL     = 30 * time.Second
	localAPIStatusURL   = "http://local-tailscaled.sock/localapi/v0/status"
	localAPIMaxBodySize = 4 << 20
)

// Device is one IP address resolved to a device name.
type Device struct {
	IP       string `json:"ip"`
	Name     string `json:"name"`
	HostName string `json:"host_name,omitempty"`
	OS       string `json:"os,omitempty"`
	Online   bool   `json:"online"`
	Self     bool   `json:"self,omitempty"`
	Source   string `json:"source"`
}

// Snapshot is the resolved device list returned to the admin UI.
type Snapshot struct {
	Devices            []Device `json:"devices"`
	TailscaleAvailable bool     `json:"tailscale_available"`
	TailscaleError     string   `json:"tailscale_error,omitempty"`
}

// Resolver caches the tailscale peer list for a short TTL.
type Resolver struct {
	socketPath string
	aliases    []Device
	ttl        time.Duration
	client     *http.Client

	mu       sync.Mutex
	cached   *Snapshot
	cachedAt time.Time
}

// NewResolver creates a resolver. An empty socketPath disables tailscale lookup.
func NewResolver(socketPath, aliasSpec string) *Resolver {
	socketPath = strings.TrimSpace(socketPath)
	r := &Resolver{
		socketPath: socketPath,
		aliases:    ParseAliases(aliasSpec),
		ttl:        defaultCacheTTL,
	}
	if socketPath != "" {
		dialer := &net.Dialer{Timeout: 2 * time.Second}
		r.client = &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, "unix", socketPath)
				},
			},
		}
	}
	return r
}

// ParseAliases parses "ip=name,ip=name". Invalid entries are skipped.
func ParseAliases(spec string) []Device {
	var out []Device
	for _, entry := range strings.Split(spec, ",") {
		ipPart, name, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(ipPart))
		name = strings.TrimSpace(name)
		if ip == nil || name == "" {
			continue
		}
		out = append(out, Device{IP: ip.String(), Name: name, Source: SourceAlias})
	}
	return out
}

// Snapshot returns the merged device list. Tailscale failures are reported in
// the snapshot instead of returned as errors so aliases keep working.
func (r *Resolver) Snapshot(ctx context.Context) Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached != nil && time.Since(r.cachedAt) < r.ttl {
		return *r.cached
	}

	snap := Snapshot{}
	var tailscaleDevices []Device
	if r.client != nil {
		devices, err := r.fetchTailscale(ctx)
		if err != nil {
			snap.TailscaleError = err.Error()
			logger.LegacyPrintf("ipdevice", "[IPDevice] tailscale status unavailable: %v", err)
		} else {
			snap.TailscaleAvailable = true
			tailscaleDevices = devices
		}
	}
	snap.Devices = mergeDevices(tailscaleDevices, r.aliases)

	r.cached = &snap
	r.cachedAt = time.Now()
	return snap
}

type localAPIPeer struct {
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	OS           string   `json:"OS"`
	Online       bool     `json:"Online"`
	TailscaleIPs []string `json:"TailscaleIPs"`
}

type localAPIStatus struct {
	Self *localAPIPeer           `json:"Self"`
	Peer map[string]localAPIPeer `json:"Peer"`
}

func (r *Resolver) fetchTailscale(ctx context.Context) ([]Device, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, localAPIStatusURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query tailscaled: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tailscaled status: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, localAPIMaxBodySize))
	if err != nil {
		return nil, fmt.Errorf("read tailscaled status: %w", err)
	}
	return parseLocalAPIStatus(body)
}

func parseLocalAPIStatus(body []byte) ([]Device, error) {
	var status localAPIStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("decode tailscaled status: %w", err)
	}
	var out []Device
	appendPeer := func(p localAPIPeer, self bool) {
		name := strings.TrimSpace(strings.SplitN(strings.TrimSuffix(p.DNSName, "."), ".", 2)[0])
		if name == "" {
			name = strings.TrimSpace(p.HostName)
		}
		if name == "" {
			return
		}
		for _, raw := range p.TailscaleIPs {
			ip := net.ParseIP(strings.TrimSpace(raw))
			if ip == nil {
				continue
			}
			out = append(out, Device{
				IP:       ip.String(),
				Name:     name,
				HostName: p.HostName,
				OS:       p.OS,
				Online:   p.Online || self,
				Self:     self,
				Source:   SourceTailscale,
			})
		}
	}
	if status.Self != nil {
		appendPeer(*status.Self, true)
	}
	for _, p := range status.Peer {
		appendPeer(p, false)
	}
	return out, nil
}

// mergeDevices lets aliases override tailscale names for the same IP and
// returns a stable order (by name, then IP).
func mergeDevices(tailscale, aliases []Device) []Device {
	byIP := make(map[string]Device, len(tailscale)+len(aliases))
	for _, d := range tailscale {
		byIP[d.IP] = d
	}
	for _, a := range aliases {
		if existing, ok := byIP[a.IP]; ok {
			existing.Name = a.Name
			existing.Source = SourceAlias
			byIP[a.IP] = existing
			continue
		}
		byIP[a.IP] = a
	}
	out := make([]Device, 0, len(byIP))
	for _, d := range byIP {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return out[i].IP < out[j].IP
	})
	return out
}
