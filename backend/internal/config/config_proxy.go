package config

import (
	"fmt"
	"net"
)

// DefaultTrustedProxyCIDR is the range Railway's edge proxies connect from.
// Production logs showed every peer_ip inside 100.64.0.0/10 (CGNAT).
const DefaultTrustedProxyCIDR = "100.64.0.0/10"

// ProxyConfig describes which connecting peers may speak for a client.
type ProxyConfig struct {
	// TrustedCIDRs are the only peers whose X-Real-IP is believed. Loopback,
	// link-local and private ranges are not trusted unless listed here.
	TrustedCIDRs []*net.IPNet
	// UsedDefault is true when TRUSTED_PROXY_CIDRS was not set.
	UsedDefault bool
}

func (cfg *Config) proxyConfig() error {
	trusted, usedDefault, err := trustedProxyCIDRs()
	if err != nil {
		return err
	}
	cfg.Proxy = ProxyConfig{TrustedCIDRs: trusted, UsedDefault: usedDefault}
	return nil
}

// trustedProxyCIDRs reads TRUSTED_PROXY_CIDRS, a comma-separated list of CIDR
// ranges. It falls back to DefaultTrustedProxyCIDR and reports that it did.
func trustedProxyCIDRs() ([]*net.IPNet, bool, error) {
	raw := getEnvSlice("TRUSTED_PROXY_CIDRS", nil)
	usedDefault := len(raw) == 0
	if usedDefault {
		raw = []string{DefaultTrustedProxyCIDR}
	}

	ranges := make([]*net.IPNet, 0, len(raw))
	for _, s := range raw {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, false, fmt.Errorf("TRUSTED_PROXY_CIDRS: invalid CIDR %q: %w", s, err)
		}
		ranges = append(ranges, n)
	}
	return ranges, usedDefault, nil
}
