package auth

import (
	"net/netip"
	"testing"
)

func TestIPLimitKey(t *testing.T) {
	tests := []struct {
		name, ip, want string
	}{
		{"IPv4 by address", "203.0.113.7", "ip:203.0.113.7"},
		{"IPv6 by /64", "2001:db8:1:2:aaaa:bbbb:cccc:dddd", "ip:2001:db8:1:2::/64"},
		{"same /64, same key", "2001:db8:1:2::1", "ip:2001:db8:1:2::/64"},
		{"zone dropped", "fe80::1%eth0", "ip:fe80::/64"},
		{"unknown address", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ip netip.Addr
			if tt.ip != "" {
				ip = netip.MustParseAddr(tt.ip)
			}
			if got := ipLimitKey(keyPrefixLoginIP, ip); got != tt.want {
				t.Errorf("ipLimitKey(%s) = %q, want %q", tt.ip, got, tt.want)
			}
		})
	}
}
