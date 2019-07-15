package cache

import (
	"testing"

	"github.com/miekg/dns"
)

func TestCache(t *testing.T) {
	cache := New(Config{ExpirationTime: 1000})
	cache.Set("1", Element{Value: []dns.RR{}})
	cache.Get("1")
}
