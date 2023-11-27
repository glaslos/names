package names

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func (n *Names) newTLSConnection(ctx context.Context, network, address string, timeout time.Duration) (*dns.Conn, error) {
	tlsDialer := tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout:   timeout,
			KeepAlive: 15 * time.Second,
		},
		Config: &tls.Config{},
	}

	// attempt TLS connection
	conn, err := tlsDialer.DialContext(ctx, network, address)
	if err != nil {
		n.Log.Warn().Err(err).Msg("failed to dial")
	} else {
		return &dns.Conn{Conn: conn}, nil
	}
	// fallback to UDP
	return dns.DialTimeout("udp", strings.Split(address, ":")[0]+":53", timeout)
}
