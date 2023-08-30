package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/db"
	"github.com/ViBiOh/httputils/v4/pkg/hash"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/jackc/pgx/v5"
)

func main() {
	fs := flag.NewFlagSet("indexer", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	inputFile := flags.New("input", "JSON File").DocPrefix("indexer").String(fs, "", nil)
	language := flags.New("language", "Language for tsvector").DocPrefix("indexer").String(fs, "french", nil)

	dbConfig := db.Flags(fs, "db")

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	quoteDB, err := db.New(ctx, dbConfig, nil)
	if err != nil {
		slog.Error("create db", "err", err)
		os.Exit(1)
	}

	defer quoteDB.Close()

	quotes, collectionName, err := readQuotes(*inputFile)
	if err != nil {
		slog.Error("read quote", "err", err)
		os.Exit(1)
	}

	if err := quoteDB.DoAtomic(ctx, func(ctx context.Context) error {
		collectionID, err := getOrCreateCollection(ctx, quoteDB, collectionName, *language)
		if err != nil {
			return fmt.Errorf("get or create collection: %w", err)
		}

		if err := quoteDB.Exec(ctx, "DELETE FROM kaamebott.quote WHERE collection_id = $1", collectionID); err != nil {
			return fmt.Errorf("delete collection: %w", err)
		}

		if err := insertQuotes(ctx, quoteDB, collectionID, quotes); err != nil {
			return fmt.Errorf("insert quotes: %w", err)
		}

		if err := quoteDB.Exec(ctx, fmt.Sprintf("UPDATE kaamebott.quote SET search_vector = to_tsvector('%s', id) || to_tsvector('%s', value) || to_tsvector('%s', character) || to_tsvector('%s', context) WHERE collection_id = $1;", *language, *language, *language, *language), collectionID); err != nil {
			return fmt.Errorf("create search vector for quote: %w", err)
		}

		return nil
	}); err != nil {
		slog.Error("update quotes", "err", err)
		os.Exit(1)
	}

	slog.Info("Collection indexed", "collection", collectionName)
}

func readQuotes(filename string) ([]model.Quote, string, error) {
	reader, err := os.OpenFile(filename, os.O_RDONLY, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open file: %w", err)
	}

	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			slog.Error("close", "err", closeErr, "item", filename, "fn", "indexer.readQuotes")
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
