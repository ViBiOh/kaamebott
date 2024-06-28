package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/db"
	"github.com/ViBiOh/httputils/v4/pkg/hash"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/jackc/pgx/v5"
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
}

func main() {
	fs := flag.NewFlagSet("indexer", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	inputFile := flags.New("input", "JSON File").DocPrefix("indexer").String(fs, "", nil)
	language := flags.New("language", "Language for tsvector").DocPrefix("indexer").String(fs, "french", nil)
	enrich := flags.New("enrich", "Enrich existing collection").DocPrefix("indexer").Bool(fs, false, nil)

	dbConfig := db.Flags(fs, "db")

	_ = fs.Parse(os.Args[1:])

	ctx := context.Background()

	quoteDB, err := db.New(ctx, dbConfig, nil)
	logger.FatalfOnErr(ctx, err, "create db")

	defer quoteDB.Close()

	quotes, collectionName, err := readQuotes(ctx, *inputFile)
	logger.FatalfOnErr(ctx, err, "read quote")

	collectionID, err := getOrCreateCollection(ctx, quoteDB, collectionName, *language)
	logger.FatalfOnErr(ctx, err, "get or create collection")

	if !*enrich {
		logger.FatalfOnErr(ctx, replaceQuotes(ctx, quoteDB, collectionID, language, quotes), "replace collection")
	} else {
		logger.FatalfOnErr(ctx, enrichQuotes(ctx, quoteDB, collectionID, language, quotes), "replace collection")
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "Collection indexed", slog.String("collection", collectionName))
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

	return quotes, path.Base(strings.TrimSuffix(reader.Name(), ".json")), nil
}

func getOrCreateCollection(ctx context.Context, quoteDB db.Service, name, language string) (uint64, error) {
	var collectionID uint64

	if err := quoteDB.Get(ctx, func(row pgx.Row) error {
		err := row.Scan(&collectionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}, "SELECT id FROM kaamebott.collection WHERE name = $1", name); err != nil {
		return collectionID, fmt.Errorf("get collection `%s`: %w", name, err)
	}

	if collectionID != 0 {
		return collectionID, nil
	}

	id, err := quoteDB.Create(ctx, "INSERT INTO kaamebott.collection (name, language) VALUES ($1, $2) RETURNING id", name, language)
	if err != nil {
		return collectionID, fmt.Errorf("create collection: %w", err)
	}

	return id, nil
}

func replaceQuotes(ctx context.Context, quoteDB db.Service, collectionID uint64, language *string, quotes []model.Quote) error {
	return quoteDB.DoAtomic(ctx, func(ctx context.Context) error {
		if err := quoteDB.Exec(ctx, "DELETE FROM kaamebott.quote WHERE collection_id = $1", collectionID); err != nil {
			return fmt.Errorf("delete collection: %w", err)
		}

		if err := insertQuotes(ctx, quoteDB, collectionID, quotes); err != nil {
			return fmt.Errorf("insert quotes: %w", err)
		}

		if err := quoteDB.Exec(ctx, fmt.Sprintf("UPDATE kaamebott.quote SET search_vector = to_tsvector('%s', id) || to_tsvector('%s', value) || to_tsvector('%s', character) || to_tsvector('%s', context) WHERE collection_id = $1;", *language, *language, *language, *language), collectionID); err != nil {
			return fmt.Errorf("refresh search vector: %w", err)
		}

		return nil
	})
}

func insertQuotes(ctx context.Context, quoteDB db.Service, collectionID uint64, quotes []model.Quote) error {
	quotesCount, index := len(quotes), 0

	feedLine := func() ([]any, error) {
		if quotesCount == index {
			return nil, nil
		}

		item := quotes[index]
		if len(item.ID) == 0 {
			item.ID = hash.String(item.Value)
		}

		index++

		return []any{collectionID, item.ID, item.Value, item.Character, item.Context, item.URL}, nil
	}

	return quoteDB.Bulk(ctx, feedLine, "kaamebott", "quote", "collection_id", "id", "value", "character", "context", "url")
}

func enrichQuotes(ctx context.Context, quoteDB db.Service, collectionID uint64, language *string, quotes []model.Quote) error {
	existingsPerCharacter := make(map[string][]model.Quote)

	err := quoteDB.List(ctx, func(row pgx.Rows) error {
		var item model.Quote

		if err := row.Scan(&item.ID, &item.Value, &item.Character); err == pgx.ErrNoRows {
			return nil
		}

		sanitizedCharacter, err := sanitizeName(item.Character)
		logger.FatalfOnErr(ctx, err, "sanitize character")

		existingsPerCharacter[sanitizedCharacter] = append(existingsPerCharacter[sanitizedCharacter], item)

		return nil
	}, "SELECT id, value, character FROM kaamebott.quote WHERE collection_id = $1", collectionID)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	return quoteDB.DoAtomic(ctx, func(ctx context.Context) error {
		for _, quote := range quotes {
			sanitizedQuote, err := sanitizeName(quote.Value)
			logger.FatalfOnErr(ctx, err, "sanitize quote")
			var found bool

			for _, character := range strings.Split(quote.Character, ",") {
				sanitizedCharacter, err := sanitizeName(character)
				logger.FatalfOnErr(ctx, err, "sanitize character")

				if mapped, ok := characterMapping[sanitizedCharacter]; ok {
					sanitizedCharacter = mapped
				}

				for _, existing := range existingsPerCharacter[sanitizedCharacter] {
					sanitizedExisting, err := sanitizeName(existing.Value)
					logger.FatalfOnErr(ctx, err, "sanitize existing")

					if sanitizedExisting == sanitizedQuote {
						err := quoteDB.Exec(ctx, "UPDATE kaamebott.quote SET image = $1 WHERE id = $2 AND collection_id = $3", quote.Image, existing.ID, collectionID)
						logger.FatalfOnErr(ctx, err, "update quote")

						found = true
						break
					}
				}
			}

			if !found {
				err := quoteDB.Exec(ctx, "INSERT INTO kaamebott.quote (id, value, character, context, url, image, collection_id) VALUES ($1, $2, $3, $4, $5, $6, $7)", quote.ID, quote.Value, quote.Character, "", "", quote.Image, collectionID)
				logger.FatalfOnErr(ctx, err, "update quote")
			}
		}

		if err := quoteDB.Exec(ctx, fmt.Sprintf("UPDATE kaamebott.quote SET search_vector = to_tsvector('%s', id) || to_tsvector('%s', value) || to_tsvector('%s', character) || to_tsvector('%s', context) WHERE collection_id = $1;", *language, *language, *language, *language), collectionID); err != nil {
			return fmt.Errorf("create search vector for quote: %w", err)
		}

		return nil
	})
}
