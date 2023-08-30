package names

import (
	"github.com/glaslos/names/cache"
	"github.com/miekg/dns"
	"github.com/spf13/viper"
)

func (n *Names) resolv(msg *dns.Msg, upstream string, dataCh chan cache.Element, stopCh chan struct{}) {
	m, _, err := n.dnsClient.Exchange(msg, upstream)
	if err != nil {
		n.Log.Error().Err(err)
		return
	}
	select {
	case <-stopCh:
		return
	default:
		element := cache.Element{Value: m.Answer, Resolver: upstream, Request: msg}
		dataCh <- element
	}
}

func (n *Names) resolveUpstream(msg *dns.Msg) cache.Element {
	dataCh := make(chan cache.Element)
	stopCh := make(chan struct{})
	for _, upstream := range viper.GetStringSlice("upstreams") {
		go n.resolv(msg, upstream, dataCh, stopCh)
	}

	var element cache.Element

	for {
		element = <-dataCh
		close(stopCh)
		break
	}
	return element
}
