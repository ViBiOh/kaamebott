package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/hash"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/meilisearch/meilisearch-go"
)

var characterMapping = map[string]string{
	"attilachefdeshuns":   "attila",
	"breccanlartisan":     "breccan",
	"buzitlebarde":        "lebarde",
	"caiuscamillus":       "caius",
	"lancelotdulac":       "lancelot",
	"linterpreteburgonde": "linterprete",
	"lothdorcanie":        "loth",
	"mevanwi":             "mevanoui",
	"monseigneurboniface": "levequeboniface",
	"doloreskoulechov":    "dolores",
}

func main() {
	fs := flag.NewFlagSet("indexer", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	inputFile := flags.New("input", "JSON File").DocPrefix("indexer").String(fs, "", nil)
	enrich := flags.New("enrich", "JSON Enrich file").DocPrefix("indexer").String(fs, "", nil)
	searchURL := flags.New("url", "Meilisearch URL").DocPrefix("indexer").String(fs, "http://localhost:7700", nil)

	_ = fs.Parse(os.Args[1:])

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	searchClient := meilisearch.NewClient(meilisearch.ClientConfig{Host: *searchURL})

	quotes, indexName, err := readQuotes(ctx, *inputFile)
	logger.FatalfOnErr(ctx, err, "read quote")

	index, err := getIndex(ctx, searchClient, indexName)
	logger.FatalfOnErr(ctx, err, "get or create collection")

	logger.FatalfOnErr(ctx, replaceQuotes(ctx, index, quotes), "replace quotes")

	if len(*enrich) != 0 {
		enriched, _, err := readQuotes(ctx, *enrich)
		logger.FatalfOnErr(ctx, err, "read enrich")

		logger.FatalfOnErr(ctx, enrichQuotes(ctx, index, quotes, enriched), "enrich quotes")
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "Collection indexed", slog.String("collection", indexName))
}

func readQuotes(ctx context.Context, filename string) ([]model.Quote, string, error) {
	reader, err := os.OpenFile(filename, os.O_RDONLY, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open file: %w", err)
	}

	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			slog.LogAttrs(ctx, slog.LevelError, "close", slog.String("fn", "indexer.readQuotes"), slog.String("item", filename), slog.Any("error", closeErr))
		}
	}()

	var quotes []model.Quote
	if err := json.NewDecoder(reader).Decode(&quotes); err != nil {
		return nil, "", fmt.Errorf("load quotes: %w", err)
	}

	for i, quote := range quotes {
		quotes[i].ID = hash.Hash(quote)
	}

	return quotes, path.Base(strings.TrimSuffix(reader.Name(), ".json")), nil
}

func getIndex(ctx context.Context, search *meilisearch.Client, name string) (*meilisearch.Index, error) {
	createTask, err := search.CreateIndex(&meilisearch.IndexConfig{Uid: name})
	if err != nil {
		return nil, fmt.Errorf("create index: %w", err)
	}

	if _, err := search.WaitForTask(createTask.TaskUID, meilisearch.WaitParams{Context: ctx, Interval: time.Second}); err != nil {
		return nil, fmt.Errorf("wait index: %w", err)
	}

	return search.Index(name), nil
}

func replaceQuotes(ctx context.Context, index *meilisearch.Index, quotes []model.Quote) error {
	deleteTask, err := index.DeleteAllDocuments()
	if err != nil {
		return fmt.Errorf("delete documents: %w", err)
	}

	if _, err := index.WaitForTask(deleteTask.TaskUID, meilisearch.WaitParams{Context: ctx, Interval: time.Second}); err != nil {
		return fmt.Errorf("wait delete: %w", err)
	}

	addTask, err := index.AddDocuments(quotes, "id")
	if err != nil {
		return fmt.Errorf("add documents: %w", err)
	}

	if _, err := index.WaitForTask(addTask.TaskUID, meilisearch.WaitParams{Context: ctx, Interval: time.Second}); err != nil {
		return fmt.Errorf("wait add: %w", err)
	}

	return nil
}

func enrichQuotes(ctx context.Context, index *meilisearch.Index, quotes []model.Quote, enriched []model.Quote) error {
	existingsPerCharacter := make(map[string][]model.Quote)

	for _, quote := range quotes {
		sanitizedCharacter, err := sanitizeName(quote.Character)
		if err != nil {
			return fmt.Errorf("sanitize character `%s`: %w", quote.Character, err)
		}

		existingsPerCharacter[sanitizedCharacter] = append(existingsPerCharacter[sanitizedCharacter], quote)
	}

	var toAdd []model.Quote
	var toUpdate []model.Quote

	for _, quote := range enriched {
		sanitizedQuote, err := sanitizeName(quote.Value)
		if err != nil {
			return fmt.Errorf("sanitize quote: %w", err)
		}

		var found bool

		for _, character := range strings.Split(quote.Character, ",") {
			sanitizedCharacter, err := sanitizeName(character)
			if err != nil {
				return fmt.Errorf("sanitize character `%s`: %w", character, err)
			}

			if mapped, ok := characterMapping[sanitizedCharacter]; ok {
				sanitizedCharacter = mapped
			}

			for _, existing := range existingsPerCharacter[sanitizedCharacter] {
				sanitizedExisting, err := sanitizeName(existing.Value)
				if err != nil {
					return fmt.Errorf("sanitize value `%s`: %w", existing.Value, err)
				}

				if sanitizedExisting == sanitizedQuote {
					existing.Image = quote.Image

					toUpdate = append(toUpdate, existing)

					found = true
					break
				}
			}
		}

		if !found {
			toAdd = append(toAdd, quote)
		}
	}

	if len(toUpdate) != 0 {
		updateTask, err := index.UpdateDocuments(toUpdate)
		if err != nil {
			return fmt.Errorf("update quote: %w", err)
		}

		if _, err := index.WaitForTask(updateTask.TaskUID, meilisearch.WaitParams{Context: ctx, Interval: time.Second}); err != nil {
			return fmt.Errorf("wait update: %w", err)
		}
	}

	if len(toAdd) != 0 {
		addTask, err := index.AddDocuments(toAdd)
		if err != nil {
			return fmt.Errorf("add quote: %w", err)
		}

		if _, err := index.WaitForTask(addTask.TaskUID, meilisearch.WaitParams{Context: ctx, Interval: time.Second}); err != nil {
			return fmt.Errorf("wait add: %w", err)
		}
	}

	return nil
}
