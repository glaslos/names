package names

import (
	"testing"

	"github.com/glaslos/names/cache"
	"github.com/glaslos/names/lists"
)

func TestBlocklisted(t *testing.T) {
	n, err := New(&Config{LoggerConfig: &LoggerConfig{}, CacheConfig: &cache.Config{}})
	if err != nil {
		t.Fatal(err)
	}
	n.tree.Add(lists.ReverseString("google.com"))
	if !n.isBlocklisted("google.com") {
		t.Fatal("should be blocklisted")
	}
}
