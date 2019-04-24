package names

import (
	"github.com/fvbock/trie"
	"github.com/rs/zerolog"
	"testing"
)

func TestLists(t *testing.T) {
	tree := trie.NewTrie()
	logger := zerolog.Nop()
	if err := fetchLists(&logger, tree); err != nil {
		t.Fatal(err)
	}
}
