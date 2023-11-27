package names

import (
	"errors"
	"net/netip"
	"time"

	"github.com/glaslos/names/cache"

	"github.com/phuslu/fastdns"
)

func (n *Names) resolv(req *fastdns.Message, upstream *Upstream, dataCh chan cache.Element, stopCh chan struct{}) {
	resp := fastdns.AcquireMessage()
	defer fastdns.ReleaseMessage(resp)
	if err := upstream.client.Exchange(req, resp); err != nil {
		n.Log.Error().Err(err).Str("resolver", upstream.addr).Msg("failed to exchange DNS request")
		return
	}

	select {
	case <-stopCh:
		return
	default:
		_ = resp.Walk(func(name []byte, typ fastdns.Type, class fastdns.Class, ttl uint32, data []byte) bool {
			var v netip.Addr
			switch typ {
			case fastdns.TypeA, fastdns.TypeAAAA:
				v, _ = netip.AddrFromSlice(data)
				element := cache.Element{Value: v.String(), Resolver: upstream.addr, Request: req.Raw}
				dataCh <- element
			}
			return true
		})
	}
}

func (n *Names) resolveUpstream(msg *fastdns.Message) (cache.Element, error) {
	dataCh := make(chan cache.Element)
	stopCh := make(chan struct{})
	for _, upstream := range n.dnsUpstreams {
		go n.resolv(msg, upstream, dataCh, stopCh)
	}
	ticker := time.NewTicker(4 * time.Second)
	select {
	case <-ticker.C:
		return cache.Element{}, errors.New("resolve upstream timeout")
	case element := <-dataCh:
		ticker.Stop()
		close(stopCh)
		return element, nil
	}
}

func newClient(server string, port int16) (*fastdns.Client, error) {
	addr, err := netip.ParseAddr(server)
	if err != nil {
		return nil, err
	}
	client := &fastdns.Client{
		AddrPort:     netip.AddrPortFrom(addr, uint16(port)),
		ReadTimeout:  2 * time.Second,
		MaxConns:     100,
		MaxIdleConns: 20,
	}
	return client, nil
}
