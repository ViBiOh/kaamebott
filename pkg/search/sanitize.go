package search

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	transliterations = map[string]string{
		"Ð": "D",
		"Ł": "l",
		"Ø": "oe",
		"Þ": "Th",
		"ß": "ss",
		"æ": "ae",
		"ð": "d",
		"ł": "l",
		"ø": "oe",
		"þ": "th",
		"œ": "oe",
	}
	quotesChar   = regexp.MustCompile(`["'` + "`" + `](?m)`)
	specialChars = regexp.MustCompile(`[^a-z0-9.\-/ ](?m)`)

	transformerPool = sync.Pool{
		New: func() any {
			return transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
		},
	}
)

func sanitizeName(name string) (string, error) {
	withoutLigatures := strings.ToLower(name)
	for key, value := range transliterations {
		if strings.Contains(withoutLigatures, key) {
			withoutLigatures = strings.ReplaceAll(withoutLigatures, key, value)
		}
	}

	transformer := transformerPool.Get().(transform.Transformer)
	defer transformerPool.Put(transformer)

	withoutDiacritics, _, err := transform.String(transformer, withoutLigatures)
	if err != nil {
		return "", err
	}

	withoutQuotes := quotesChar.ReplaceAllString(withoutDiacritics, " ")

	return specialChars.ReplaceAllString(withoutQuotes, ""), nil
}

func getWords(query string) ([]string, error) {
	if len(query) == 0 {
		return nil, nil
	}

	sanitizedQuery, err := sanitizeName(query)
	if err != nil {
		return nil, fmt.Errorf("sanitize query: %s", err)
	}

	var words []string
	for _, word := range strings.Split(sanitizedQuery, " ") {
		if len(word) < 3 {
			continue
		}

		words = append(words, word)
	}

	return words, nil
}
