//go:build unit

package ipdevice

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleStatus = `{
  "Self": {"HostName": "us-home-prod", "DNSName": "us-home-prod.tail412eaf.ts.net.", "OS": "linux", "Online": false,
           "TailscaleIPs": ["100.93.20.57", "fd7a:115c:a1e0::333:143a"]},
  "Peer": {
    "nodekey:a": {"HostName": "xzb的MacBook Air", "DNSName": "xzbmacbook-air.tail412eaf.ts.net.", "OS": "macOS", "Online": true,
                  "TailscaleIPs": ["100.97.114.1"]},
    "nodekey:b": {"HostName": "REDMI K90 Ultra", "DNSName": "", "OS": "android", "Online": false,
                  "TailscaleIPs": ["100.117.46.29", "not-an-ip"]}
  }
}`

func TestParseAliases(t *testing.T) {
	got := ParseAliases(" 172.18.0.1 = 家宽1 本机 ,bad,=x,1.2.3.4=, 100.97.114.1=Mac ")
	assert.Equal(t, []Device{
		{IP: "172.18.0.1", Name: "家宽1 本机", Source: SourceAlias},
		{IP: "100.97.114.1", Name: "Mac", Source: SourceAlias},
	}, got)
}

func TestResolverSnapshotMergesTailscaleAndAliases(t *testing.T) {
	dir, err := os.MkdirTemp("", "tsapi")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "ts.sock")
	ln, err := net.Listen("unix", socket)
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/localapi/v0/status" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(sampleStatus))
	}))
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)

	snap := NewResolver(socket, "172.18.0.1=Caddy,100.97.114.1=My Mac").Snapshot(context.Background())

	require.True(t, snap.TailscaleAvailable)
	assert.Empty(t, snap.TailscaleError)
	assert.Equal(t, []Device{
		{IP: "172.18.0.1", Name: "Caddy", Source: SourceAlias},
		{IP: "100.97.114.1", Name: "My Mac", HostName: "xzb的MacBook Air", OS: "macOS", Online: true, Source: SourceAlias},
		{IP: "100.117.46.29", Name: "REDMI K90 Ultra", HostName: "REDMI K90 Ultra", OS: "android", Source: SourceTailscale},
		{IP: "100.93.20.57", Name: "us-home-prod", HostName: "us-home-prod", OS: "linux", Online: true, Self: true, Source: SourceTailscale},
		{IP: "fd7a:115c:a1e0::333:143a", Name: "us-home-prod", HostName: "us-home-prod", OS: "linux", Online: true, Self: true, Source: SourceTailscale},
	}, snap.Devices)
}

func TestResolverSnapshotKeepsAliasesWhenTailscaleUnavailable(t *testing.T) {
	snap := NewResolver("/nonexistent/tailscaled.sock", "172.18.0.1=Caddy").Snapshot(context.Background())

	assert.False(t, snap.TailscaleAvailable)
	assert.NotEmpty(t, snap.TailscaleError)
	assert.Equal(t, []Device{{IP: "172.18.0.1", Name: "Caddy", Source: SourceAlias}}, snap.Devices)
}
