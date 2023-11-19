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
	rr, err := dns.NewRR("test. 3600 IN A 127.0.0.1")
	require.NoError(t, err)
	element := &Element{Value: rr.String()}
	cache.Set("1", *element)
	var ok bool
	element, ok = cache.Get("1")
	require.True(t, ok)
	require.NotNil(t, element)
}

func TestSaveAndLoad(t *testing.T) {
	config := Config{ExpirationTime: 10 * time.Second}
	cache, err := New(config)
	require.NoError(t, err)
	rr, err := dns.NewRR("test. 3600 IN A 127.0.0.1")
	require.NoError(t, err)
	element := &Element{Value: rr.String()}
	cache.Set("1", *element)
	err = cache.Save("test.dump")
	require.NoError(t, err)
	defer func() {
		err = os.Remove("test.dump")
		require.NoError(t, err)
	}()
	cache, err = New(config)
	require.NoError(t, err)
	err = cache.Load("test.dump")
	require.NoError(t, err)
	var ok bool
	element, ok = cache.Get("1")
	require.True(t, ok)
	require.NotNil(t, element)
	require.NotEmpty(t, element.Value)
	require.Equal(t, "test.\t3600\tIN\tA\t127.0.0.1", element.Value)
}
