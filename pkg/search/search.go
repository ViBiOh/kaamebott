package search

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/kaamebott/pkg/indexer"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/meilisearch/meilisearch-go"
)

var (
	ErrNotFound      = errors.New("no result found")
	ErrIndexNotFound = errors.New("index not found")
	FuncMap          = template.FuncMap{}
)

type Service struct {
	random   *rand.Rand
	renderer *renderer.Service
	search   meilisearch.ServiceManager
}

type Config struct {
	URL string
}

func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) *Config {
	var config Config

	flags.New("URL", "Meilisearch URL").Prefix(prefix).DocPrefix("search").StringVar(fs, &config.URL, "http://meilisearch:7700", overrides)

	return &config
}

func New(config *Config, rendererService *renderer.Service) Service {
	return Service{
		search:   meilisearch.New(config.URL),
		random:   rand.New(rand.NewPCG(uint64(time.Now().Unix()), uint64(time.Now().UnixMicro()))),
		renderer: rendererService,
	}
}

func (s Service) HasIndex(ctx context.Context, indexName string) bool {
	index, err := s.search.GetIndex(indexName)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, fmt.Sprintf("check if index `%s` exists", indexName), slog.Any("error", err))
	}

	return index != nil
}

func (s Service) GetByID(ctx context.Context, indexName, id string) (model.Quote, error) {
	index, err := s.search.GetIndex(indexName)
	if err != nil {
		return model.Quote{}, err
	}

	var output model.Quote

	return output, index.GetDocument(id, &meilisearch.DocumentQuery{}, &output)
}

func (s Service) Search(ctx context.Context, indexName, query string, offset int) (model.Quote, error) {
	index, err := s.search.GetIndex(indexName)
	if err != nil {
		var meiliError *meilisearch.Error
		if errors.As(err, &meiliError) && meiliError.StatusCode == http.StatusNotFound {
			go func(ctx context.Context) {
				if indexErr := indexer.Index(ctx, s.search, indexName); indexErr != nil {
					slog.LogAttrs(ctx, slog.LevelError, fmt.Sprintf("fail to index `%s`", indexName), slog.Any("error", indexErr))
				}
			}(context.WithoutCancel(ctx))

			return model.Quote{}, ErrIndexNotFound
		}

		return model.Quote{}, fmt.Errorf("get index: %w", err)
	}

	results, err := index.Search(query, &meilisearch.SearchRequest{Limit: 1, Offset: int64(offset)})
	if err != nil {
		return model.Quote{}, err
	}

	if len(results.Hits) == 0 {
		return model.Quote{}, ErrNotFound
	}

	var output model.Quote

	var content map[string]any
	if err := results.Hits[0].DecodeInto(&content); err != nil {
		return model.Quote{}, err
	}

	return output, index.GetDocument(content["id"].(string), &meilisearch.DocumentQuery{}, &output)
}

func (s Service) Random(ctx context.Context, indexName string) (model.Quote, error) {
	return s.Search(ctx, indexName, "", 0)
}

func (s Service) TemplateFunc(w http.ResponseWriter, r *http.Request) (renderer.Page, error) {
	return renderer.NewPage("public", http.StatusOK, nil), nil
}
