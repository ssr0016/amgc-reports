package config

import (
	"fmt"
	"net"
)

// DefaultTrustedProxyCIDR trusts X-Forwarded-For from every IPv4 peer. It is only a
// convenience outside production: ValidateProxy refuses it there, because with
// it any client chooses its own address for rate limits and audit rows.
const DefaultTrustedProxyCIDR = "0.0.0.0/0"

// ProxyConfig describes which connecting peers may speak for a client.
type ProxyConfig struct {
	// TrustedCIDRs are the only peers whose X-Forwarded-For is believed. Loopback,
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

// ValidateProxy fails closed in production: TRUSTED_PROXY_CIDRS must be set and
// must not contain a /0 range (0.0.0.0/0, ::/0), which would trust every peer.
// The server calls it at startup; like ValidateAuth it is not part of Validate,
// so `api migrate`, which serves no requests, does not need the variable.
func (c *Config) ValidateProxy() error {
	if !c.IsProduction() {
		return nil
	}
	if c.Proxy.UsedDefault {
		return fmt.Errorf("TRUSTED_PROXY_CIDRS is required in production: set it to the proxy range in front of the service")
	}
	for _, n := range c.Proxy.TrustedCIDRs {
		if ones, _ := n.Mask.Size(); ones == 0 {
			return fmt.Errorf("TRUSTED_PROXY_CIDRS: %s trusts every peer, which lets any client choose its own IP; not allowed in production", n)
		}
	}
	return nil
}
