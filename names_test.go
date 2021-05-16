package names

import (
	"testing"

	"github.com/glaslos/names/cache"
)

func TestBlacklisted(t *testing.T) {
	n, err := New(&Config{LoggerConfig: &LoggerConfig{}, CacheConfig: &cache.Config{}})
	if err != nil {
		t.Fatal(err)
	}
	n.tree.Add(reverseString("google.com"))
	if !n.isBlacklisted("google.com") {
		t.Fatal("should be blacklisted")
	}
}
