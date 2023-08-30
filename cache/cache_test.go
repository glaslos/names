package cache

import (
	"os"
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

func TestSaveAndLoad(t *testing.T) {
	cache, err := New(Config{ExpirationTime: 10 * time.Second})
	require.NoError(t, err)
	element := &Element{Value: []dns.RR{}}
	cache.Set("1", *element)
	err = cache.Save("test.dump")
	require.NoError(t, err)
	defer func() {
		err = os.Remove("test.dump")
		require.NoError(t, err)
	}()
	cache, err = New(Config{ExpirationTime: 10 * time.Second})
	require.NoError(t, err)
	err = cache.Load("test.dump")
	require.NoError(t, err)
	var ok bool
	element, ok = cache.Get("1")
	require.True(t, ok)
	require.NotNil(t, element)
}
