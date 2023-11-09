package names

import (
	"testing"

	"github.com/glaslos/names/cache"
	"github.com/glaslos/names/lists"

	"github.com/stretchr/testify/require"
)

func TestIsBlocklisted(t *testing.T) {
	n, err := New(&Config{LoggerConfig: &LoggerConfig{}, CacheConfig: &cache.Config{}})
	require.NoError(t, err)

	n.tree.Add(lists.ReverseString("google.com"))
	require.True(t, n.isBlocklisted("google.com"))
}
