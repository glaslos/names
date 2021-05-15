package cache

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	cache, err := New(Config{ExpirationTime: 1000})
	require.NoError(t, err)
	cache.Set("1", Element{Value: []dns.RR{}})
	cache.Get("1")
}
