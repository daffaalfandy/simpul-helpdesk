package httpapi

import (
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.1/32")}
	tests := []struct {
		name    string
		remote  string
		xff     []string
		trusted []netip.Prefix
		want    string
	}{
		{"direct client", "203.0.113.7:5000", nil, proxies, "203.0.113.7"},
		{"untrusted peer cannot spoof", "203.0.113.7:5000", []string{"1.2.3.4"}, proxies, "203.0.113.7"},
		{"trusted proxy forwards client", "10.1.2.3:443", []string{"198.51.100.9"}, proxies, "198.51.100.9"},
		{"right-most untrusted hop wins", "10.1.2.3:443", []string{"1.2.3.4, 198.51.100.9, 10.9.9.9"}, proxies, "198.51.100.9"},
		{"multiple headers", "127.0.0.1:1", []string{"1.2.3.4", "198.51.100.9"}, proxies, "198.51.100.9"},
		{"all hops trusted", "10.1.2.3:443", []string{"10.0.0.5"}, proxies, "10.0.0.5"},
		{"no proxies configured", "10.1.2.3:443", []string{"198.51.100.9"}, nil, "10.1.2.3"},
		{"ipv4-mapped ipv6", "[::ffff:203.0.113.7]:80", nil, nil, "203.0.113.7"},
		{"garbage in header ignored", "10.1.2.3:443", []string{"nonsense, 198.51.100.9"}, proxies, "198.51.100.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClientIP(tt.remote, tt.xff, tt.trusted); got.String() != tt.want {
				t.Fatalf("ClientIP = %s, want %s", got, tt.want)
			}
		})
	}
}
