package names

import (
	"context"
	"testing"

	"github.com/glaslos/names/cache"
	"github.com/glaslos/names/lists"

	"github.com/stretchr/testify/require"
)

func TestIsBlocklisted(t *testing.T) {
	cfg := &Config{LoggerConfig: &LoggerConfig{}, CacheConfig: &cache.Config{RefreshCache: false}}
	n, err := New(context.Background(), cfg)
	require.NoError(t, err)

	n.tree.Add(lists.ReverseString("google.com"))
	require.True(t, n.isBlocklisted("google.com"))
}
