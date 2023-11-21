package names

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/miekg/dns"
)

func (n *Names) aliveLoop(dnsUpstreams []*dns.Conn) {
	for _, upstream := range dnsUpstreams {
		alive, err := isAlive(upstream)
		if err != nil {
			n.Log.Error().Err(err)
			continue
		}
		if !alive {
			n.Log.Error().Msgf("closed connection: %s", upstream.Conn.RemoteAddr().String())
		}
	}
}

func isAlive(conn net.Conn) (bool, error) {
	var buf = [1]byte{}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
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

func newConnection(ctx context.Context, network, address string, timeout time.Duration) (*dns.Conn, error) {
	tlsDialer := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: timeout},
		Config:    &tls.Config{},
	}
	conn, err := tlsDialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	// if err := conn.(*net.TCPConn).SetKeepAlive(true); err != nil {
	// 	return nil, err
	// }
	// if err := conn.(*net.TCPConn).SetKeepAlivePeriod(30 * time.Second); err != nil {
	// 	return nil, err
	// }
	return &dns.Conn{Conn: conn}, nil
}
