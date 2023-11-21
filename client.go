package names

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/glaslos/names/cache"

	"github.com/miekg/dns"
)

func NewClient(network, address string, timeout time.Duration) (*dns.Conn, error) {
	conf := &tls.Config{}
	return dns.DialTimeoutWithTLS(network, address, conf, timeout)
}

func (n *Names) resolv(msg *dns.Msg, upstream *dns.Conn, dataCh chan cache.Element, stopCh chan struct{}) {
	if err := upstream.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		n.Log.Error().Err(err)
		return
	}
	if err := upstream.WriteMsg(msg); err != nil {
		n.Log.Error().Err(err)
		return
	}
	if err := upstream.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		n.Log.Error().Err(err)
		return
	}
	m, err := upstream.ReadMsg()
	if err != nil {
		n.Log.Error().Err(err)
		return
	}
	if len(m.Answer) == 0 {
		return
	}
	select {
	case <-stopCh:
		return
	default:
		buff, err := msg.Pack()
		if err != nil {
			n.Log.Error().Err(err)
			return
		}
		element := cache.Element{Value: m.Answer[0].String(), Resolver: upstream.RemoteAddr().String(), Request: buff}
		dataCh <- element
	}
}

func (n *Names) resolveUpstream(msg *dns.Msg) (cache.Element, error) {
	dataCh := make(chan cache.Element)
	stopCh := make(chan struct{})
	for _, upstream := range n.dnsUpstreams {
		go n.resolv(msg, upstream, dataCh, stopCh)
	}
	ticker := time.NewTicker(4 * time.Second)
	select {
	case <-ticker.C:
		return cache.Element{}, errors.New("resolve timeout")
	case element := <-dataCh:
		close(stopCh)
		return element, nil
	}
}
