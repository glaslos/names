package names

import (
	"bufio"
	"net/http"
	"strings"
	"unicode"

	"github.com/glaslos/trie"
	"github.com/rs/zerolog"
)

func reverse(runes []rune, length int) {
	for i, j := 0, length-1; i < length/2; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
}

// isMark determines whether the rune is a marker
func isMark(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Mc, r)
}

// ReverseString reverses the input string while respecting UTF8 encoding and combined characters
func ReverseString(text string) string {
	textRunes := []rune(text)
	textRunesLength := len(textRunes)
	if textRunesLength <= 1 {
		return text
	}

	i, j := 0, 0
	for i < textRunesLength && j < textRunesLength {
		j = i + 1
		for j < textRunesLength && isMark(textRunes[j]) {
			j++
		}

		if isMark(textRunes[j-1]) {
			// Reverses Combined Characters
			reverse(textRunes[i:j], j-i)
		}

		i = j
	}

	// Reverses the entire array
	reverse(textRunes, textRunesLength)

	return string(textRunes)
}

func (n *Names) isBlacklisted(name string) bool {
	return n.tree.Has(ReverseString(strings.Trim(name, ".")))
}

func dumpLists(tree *trie.Trie) error {
	return tree.DumpToFile("lists.dump")
}

func fetchLists(log *zerolog.Logger, tree *trie.Trie) error {
	var err error
	tree, err = trie.LoadFromFile("lists.dump")
	if err != nil {
		log.Error().Err(err)
		tree = trie.NewTrie()
	}
	resp, err := http.Get("http://sysctl.org/cameleon/hosts")
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Split(line, "#")[0]
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				line = fields[1]
			} else {
				line = fields[0]
			}
			line = ReverseString(strings.Trim(line, "."))
			if !tree.Has(line) {
				tree.Add(line)
			}
		}
	}
	return dumpLists(tree)
}
