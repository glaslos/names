package names

import (
	"crypto/tls"
	"time"

	"github.com/glaslos/names/cache"

	"github.com/miekg/dns"
)

func NewClient(network, address string, timeout time.Duration) (*dns.Conn, error) {
	conf := &tls.Config{}
	return dns.DialTimeoutWithTLS(network, address, conf, timeout)
}

func (n *Names) resolv(msg *dns.Msg, upstream *dns.Conn, dataCh chan cache.Element, stopCh chan struct{}) {
	if err := upstream.WriteMsg(msg); err != nil {
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
		element := cache.Element{Value: m.Answer[0].String(), Resolver: upstream.RemoteAddr().String(), Request: msg}
		dataCh <- element
	}
}

func (n *Names) resolveUpstream(msg *dns.Msg) (cache.Element, error) {
	dataCh := make(chan cache.Element)
	stopCh := make(chan struct{})
	for _, upstream := range n.dnsUpstreams {
		go n.resolv(msg, upstream, dataCh, stopCh)
	}
	element := <-dataCh
	close(stopCh)
	return element, nil
}
