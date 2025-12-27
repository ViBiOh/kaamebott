package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kaamebott/pkg/indexer"
	"github.com/meilisearch/meilisearch-go"
)

func main() {
	fs := flag.NewFlagSet("indexer", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	indexName := flags.New("name", "Index Name").DocPrefix("indexer").String(fs, "", nil)
	searchURL := flags.New("url", "Meilisearch URL").DocPrefix("indexer").String(fs, "http://127.0.0.1:7700", nil)

	_ = fs.Parse(os.Args[1:])

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	searchClient := meilisearch.New(*searchURL)

	logger.FatalfOnErr(ctx, indexer.Index(ctx, searchClient, *indexName), "index")

	slog.LogAttrs(ctx, slog.LevelInfo, "Collection indexed", slog.String("collection", *indexName))
}
