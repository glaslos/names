package names

import (
	"github.com/miekg/dns"
)

func (n *Names) resolv(msg *dns.Msg, upstream string, dataCh chan Element, stopCh chan struct{}) {
	m, _, err := n.dnsClient.Exchange(msg, upstream)
	if err != nil {
		n.Log.Error().Err(err)
		return
	}
	select {
	case <-stopCh:
		return
	default:
		element := Element{Value: m.Answer, Resolver: upstream}
		dataCh <- element
	}
}

func (n *Names) resolveUpstream(msg *dns.Msg) Element {
	dataCh := make(chan Element)
	stopCh := make(chan struct{})
	upstreams := []string{"1.1.1.1:853", "9.9.9.9:853", "1.0.0.1:853", "8.8.4.4:853", "8.8.8.8:853"}
	for _, upstream := range upstreams {
		go n.resolv(msg, upstream, dataCh, stopCh)
	}

	element := Element{}

	for {
		element = <-dataCh
		close(stopCh)
		break
	}
	return element
}
