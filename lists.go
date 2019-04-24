package names

import (
	"bufio"
	"net/http"
	"strings"

	"github.com/fvbock/trie"
	"github.com/rs/zerolog"
)

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
			if !tree.Has(line) {
				tree.Add(line)
			}
		}
	}
	return dumpLists(tree)
}
