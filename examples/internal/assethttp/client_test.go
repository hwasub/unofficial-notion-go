package assethttp

import (
	"context"
	"net/netip"
	"testing"
)

func TestPublicDialRejectsMixedAndPrivateDNS(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "::ffff:127.0.0.1", "100.64.0.1"} {
		resolve := func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(raw)}, nil
		}
		if conn, err := publicDial(resolve)(context.Background(), "tcp", "example.com:443"); err == nil {
			conn.Close()
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestDefaultClientRejectsUnsafeSchemesAndRedirectTargets(t *testing.T) {
	c := NewClient()
	for _, raw := range []string{"http://example.com/image", "https://127.0.0.1/image", "https://example.com:444/image", "https://user:pass@example.com/image"} {
		if res, err := c.Get(raw); err == nil {
			res.Body.Close()
			t.Fatalf("accepted %s", raw)
		}
	}
}
