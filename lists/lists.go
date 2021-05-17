package lists

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
	"github.com/glaslos/trie"
	"github.com/rs/zerolog"
)

//go:embed sources.json
var sources embed.FS

type sourceConfig struct {
	Url     string `json:"url,omitempty"`
	Rule    string `json:"rule,omitempty"`
	Size    string `json:"size,omitempty"`
	Focus   string `json:"focus,omitempty"`
	Descurl string `json:"descurl,omitempty"`
}

func Dump(tree *trie.Trie) error {
	return tree.DumpToFile("lists.dump")
}

func Load() (*trie.Trie, error) {
	return trie.LoadFromFile("lists.dump")
}

func DecodeConfig() (map[string]sourceConfig, error) {
	data, err := sources.Open("sources.json")
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(data)
	sourcesList := map[string]sourceConfig{}
	err = dec.Decode(&sourcesList)
	return sourcesList, err
}

func PopulateCache(tree *trie.Trie, lists []string, log *zerolog.Logger) error {
	sourcesList, err := DecodeConfig()
	if err != nil {
		return err
	}
	for _, listName := range lists {
		log.Debug().Str("source", listName).Msg("fetching list")
		source, ok := sourcesList[listName]
		if !ok {
			log.Debug().Str("source", listName).Msg("didn't find list")
			continue
		}
		prog, err := parser.ParseProgram([]byte(source.Rule), nil)
		if err != nil {
			return err
		}
		resp, err := http.Get(source.Url)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		config := &interp.Config{
			Stdin:  resp.Body,
			Output: &buf,
		}
		_, err = interp.ExecProgram(prog, config)
		if err != nil {
			return err
		}

		var count = 0
		scanner := bufio.NewScanner(&buf)
		for scanner.Scan() {
			line := scanner.Text()
			if scanner.Err() != nil {
				log.Err(err).Msg("failed to read line")
				break
			}
			line = strings.Trim(line, " \n")
			line = ReverseString(strings.Trim(line, "."))
			if !tree.Has(line) {
				tree.Add(line)
				count++
			}
		}
		log.Debug().Str("source", listName).Msgf("added %d new block domains", count)
	}
	return nil
}
