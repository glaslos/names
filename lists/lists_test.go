package lists

import (
	"testing"

	"github.com/glaslos/trie"
	"github.com/stretchr/testify/require"
)

func TestLists(t *testing.T) {
	_, err := Load()
	require.NoError(t, err)
}

func TestContaints(t *testing.T) {
	tree := trie.NewTrie()
	tree.Add(ReverseString("google.com"))
	require.True(t, tree.Has(ReverseString("google.com")))
}

func TestPrefix(t *testing.T) {
	tree := trie.NewTrie()
	tree.Add(ReverseString("*.google.com"))
	require.True(t, len(tree.PrefixMembers(ReverseString("google.com"))) >= 0)
}

func BenchmarkTrieHas(b *testing.B) {
	tree := trie.NewTrie()
	tree.Add("google.com")
	for n := 0; n < b.N; n++ {
		if !tree.Has("google.com") {
			b.Fatal("expected hit")
		}
	}
}

func BenchmarkTriePrefix(b *testing.B) {
	tree := trie.NewTrie()
	tree.Add("google.com")
	for n := 0; n < b.N; n++ {
		if len(tree.PrefixMembers("google.com")) <= 0 {
			b.Fatal("expected hit")
		}
	}
}
