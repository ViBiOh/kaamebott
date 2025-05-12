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

	inputFile := flags.New("name", "JSON File name").DocPrefix("indexer").String(fs, "", nil)
	searchURL := flags.New("url", "Meilisearch URL").DocPrefix("indexer").String(fs, "http://localhost:7700", nil)

	_ = fs.Parse(os.Args[1:])

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	searchClient := meilisearch.New(*searchURL)

	logger.FatalfOnErr(ctx, indexer.Index(ctx, searchClient, *inputFile), "index")

	slog.LogAttrs(ctx, slog.LevelInfo, "Collection indexed", slog.String("collection", *inputFile))
}
