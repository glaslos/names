package cache

import (
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	cache, err := New(Config{ExpirationTime: 10 * time.Second})
	require.NoError(t, err)
	element := &Element{Value: []dns.RR{}}
	cache.Set("1", *element)
	var ok bool
	element, ok = cache.Get("1")
	require.True(t, ok)
	require.NotNil(t, element)
}
