package indexer

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/ViBiOh/httputils/v4/pkg/hash"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/meilisearch/meilisearch-go"
)

var (
	id          = "id"
	indexFolder = "indexes"
)

//go:embed indexes
var fs embed.FS

func Index(ctx context.Context, searchClient meilisearch.ServiceManager, name string) error {
	filename := name + ".json"

	quotes, indexName, err := readQuotes(ctx, filename)
	if err != nil {
		return fmt.Errorf("read quote for `%s`: %w", filename, err)
	}

	index, err := getIndex(ctx, searchClient, indexName)
	if err != nil {
		return fmt.Errorf("get index: %w", err)
	}

	if err := replaceQuotes(ctx, index, quotes); err != nil {
		return fmt.Errorf("replace quotes: %w", err)
	}

	enriched, _, err := readQuotes(ctx, name+"_next.json")
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelWarn, "load quote enrichment", slog.String("name", name), slog.Any("error", err))
	}

	if err := enrichQuotes(ctx, index, quotes, enriched); err != nil {
		return fmt.Errorf("enrich quotes: %w", err)
	}

	return nil
}

func readQuotes(ctx context.Context, filename string) ([]model.Quote, string, error) {
	indexFile := path.Join(indexFolder, filename)

	reader, err := fs.Open(indexFile)
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

	return quotes, path.Base(strings.TrimSuffix(indexFile, ".json")), nil
}

func getIndex(ctx context.Context, search meilisearch.ServiceManager, name string) (meilisearch.IndexManager, error) {
	createTask, err := search.CreateIndex(&meilisearch.IndexConfig{Uid: name})
	if err != nil {
		return nil, fmt.Errorf("create index: %w", err)
	}

	if _, err := search.WaitForTaskWithContext(ctx, createTask.TaskUID, time.Second); err != nil {
		return nil, fmt.Errorf("wait index: %w", err)
	}

	return search.Index(name), nil
}

func replaceQuotes(ctx context.Context, index meilisearch.IndexManager, quotes []model.Quote) error {
	deleteTask, err := index.DeleteAllDocuments(&meilisearch.DocumentOptions{})
	if err != nil {
		return fmt.Errorf("delete documents: %w", err)
	}

	if _, err := index.WaitForTaskWithContext(ctx, deleteTask.TaskUID, time.Second); err != nil {
		return fmt.Errorf("wait delete: %w", err)
	}

	addTask, err := index.AddDocuments(quotes, &meilisearch.DocumentOptions{PrimaryKey: &id})
	if err != nil {
		return fmt.Errorf("add documents: %w", err)
	}

	if _, err := index.WaitForTaskWithContext(ctx, addTask.TaskUID, time.Second); err != nil {
		return fmt.Errorf("wait add: %w", err)
	}

	return nil
}

func enrichQuotes(ctx context.Context, index meilisearch.IndexManager, quotes, enriched []model.Quote) error {
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

		for character := range strings.SplitSeq(quote.Character, ",") {
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
		updateTask, err := index.UpdateDocuments(toUpdate, &meilisearch.DocumentOptions{PrimaryKey: &id})
		if err != nil {
			return fmt.Errorf("update quote: %w", err)
		}

		if _, err := index.WaitForTaskWithContext(ctx, updateTask.TaskUID, time.Second); err != nil {
			return fmt.Errorf("wait update: %w", err)
		}
	}

	if len(toAdd) != 0 {
		addTask, err := index.AddDocuments(toAdd, &meilisearch.DocumentOptions{PrimaryKey: &id})
		if err != nil {
			return fmt.Errorf("add quote: %w", err)
		}

		if _, err := index.WaitForTaskWithContext(ctx, addTask.TaskUID, time.Second); err != nil {
			return fmt.Errorf("wait add: %w", err)
		}
	}

	return nil
}
