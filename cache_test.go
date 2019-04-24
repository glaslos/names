package names

import (
	"github.com/miekg/dns"
	"testing"
)

func TestCache(t *testing.T) {
	cache := InitCache(1000)
	cache.Set("1", Element{Value: []dns.RR{}})
	cache.Get("1")
}
