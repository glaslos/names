package names

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func (n *Names) aliveLoop(dnsUpstreams []*Upstream) {
	for _, upstream := range dnsUpstreams {
		alive, err := isAlive(upstream.conn)
		if err != nil {
			n.Log.Error().Err(err)
			continue
		}
		if !alive {
			n.Log.Warn().Msgf("closed connection: %s", upstream.conn.Conn.RemoteAddr().String())
			continue
		}
	}
}

func isAlive(conn net.Conn) (bool, error) {
	var buf = [1]byte{}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return false, err
	}
	_, err := conn.Read(buf[:])
	if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (n *Names) newConnection(ctx context.Context, network, address string, timeout time.Duration) (*dns.Conn, error) {
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
