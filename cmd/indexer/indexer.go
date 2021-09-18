package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/db"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/jackc/pgx/v4"
)

func main() {
	fs := flag.NewFlagSet("indexer", flag.ExitOnError)

	inputFile := fs.String("input", "", "JSON File")
	dbConfig := db.Flags(fs, "db")

	logger.Fatal(fs.Parse(os.Args[1:]))

	quoteDB, err := db.New(dbConfig)
	logger.Fatal(err)
	defer quoteDB.Close()

	quotes, collectionName, err := readQuotes(*inputFile)
	if err != nil {
		logger.Fatal(fmt.Errorf("unable to read quotes: %s", err))
	}

	logger.Fatal(quoteDB.DoAtomic(context.Background(), func(ctx context.Context) error {
		collectionID, err := getOrCreateCollection(ctx, quoteDB, collectionName)
		if err != nil {
			return fmt.Errorf("unable to get or create collection: %s", err)
		}

		if err := quoteDB.Exec(ctx, "DELETE FROM kaamebott.quote WHERE collection_id = $1", collectionID); err != nil {
			return fmt.Errorf("unable to delete collection: %s", err)
		}

		if err := insertQuotes(ctx, quoteDB, collectionID, quotes); err != nil {
			return fmt.Errorf("unable to insert quotes: %s", err)
		}

		if err := quoteDB.Exec(ctx, "UPDATE kaamebott.quote SET search_vector = to_tsvector('french', id) || to_tsvector('french', value) || to_tsvector('french', character) || to_tsvector('french', context);"); err != nil {
			return fmt.Errorf("unable to create search vector for quote: %s", err)
		}

		return nil
	}))

	logger.Info("Collection %s indexed", collectionName)
}

func readQuotes(filename string) ([]model.Quote, string, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0600)
	if err != nil {
		return nil, "", fmt.Errorf("unable to open file: %s", err)
	}

	var quotes []model.Quote
	if err := json.NewDecoder(file).Decode(&quotes); err != nil {
		return nil, "", fmt.Errorf("unable to load quotes: %s", err)
	}

	return quotes, path.Base(strings.TrimSuffix(file.Name(), ".json")), nil
}

func getOrCreateCollection(ctx context.Context, quoteDB db.App, name string) (uint64, error) {
	var collectionID uint64

	if err := quoteDB.Get(ctx, func(row pgx.Row) error {
		err := row.Scan(&collectionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}, "SELECT id FROM kaamebott.collection WHERE name = $1", name); err != nil {
		return collectionID, fmt.Errorf("unable to get collection `%s`: %s", name, err)
	}

	if collectionID != 0 {
		return collectionID, nil
	}

	id, err := quoteDB.Create(ctx, "INSERT INTO kaamebott.collection (name) VALUES ($1) RETURNING id", name)
	if err != nil {
		return collectionID, fmt.Errorf("unable to create collection: %s", err)
	}

	return id, nil
}

func insertQuotes(ctx context.Context, quoteDB db.App, collectionID uint64, quotes []model.Quote) error {
	quotesCount, index := len(quotes), 0

	feedLine := func() ([]interface{}, error) {
		if quotesCount == index {
			return nil, nil
		}

		item := quotes[index]
		index++

		return []interface{}{collectionID, item.ID, item.Value, item.Character, item.Context}, nil
	}

	return quoteDB.Bulk(ctx, feedLine, "kaamebott", "quote", "collection_id", "id", "value", "character", "context")
}
