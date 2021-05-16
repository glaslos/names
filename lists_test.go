package names

import (
	"testing"

	"github.com/glaslos/trie"
	"github.com/rs/zerolog"
	"gotest.tools/assert"
)

func TestLists(t *testing.T) {
	logger := zerolog.Nop()
	if _, err := loadLists(&logger, false); err != nil {
		t.Fatal(err)
	}
}

func TestReverse(t *testing.T) {
	assert.Equal(t, reverseString(""), "")
	assert.Equal(t, reverseString("X"), "X")
	assert.Equal(t, reverseString("b\u0301"), "b\u0301")
	assert.Equal(t, reverseString("😎⚽"), "⚽😎")
	assert.Equal(t, reverseString("Les Mise\u0301rables"), "selbare\u0301siM seL")
	assert.Equal(t, reverseString("ab\u0301cde"), "edcb\u0301a")
	assert.Equal(t, reverseString("This `\xc5` is an invalid UTF8 character"), "retcarahc 8FTU dilavni na si `�` sihT")
	assert.Equal(t, reverseString("The quick bròwn 狐 jumped over the lazy 犬"), "犬 yzal eht revo depmuj 狐 nwòrb kciuq ehT")
	assert.Equal(t, reverseString("google.com"), "moc.elgoog")
}

func TestContaints(t *testing.T) {
	tree := trie.NewTrie()
	tree.Add(reverseString("google.com"))
	if !tree.Has(reverseString("google.com")) {
		t.Fatal("expected entry")
	}
}

func TestPrefix(t *testing.T) {
	tree := trie.NewTrie()
	tree.Add(reverseString("*.google.com"))
	if len(tree.PrefixMembers(reverseString("google.com"))) <= 0 {
		t.Fatal("expected entry")
	}
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
