package search

import (
	"context"
	native_errors "errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/db"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/kaamebott/pkg/model"
)

// RandomQuery identify random query
const RandomQuery = "random"

var (
	// ErrNotFound occurs when no result found
	ErrNotFound = native_errors.New("no result found")

	// ErrIndexNotFound occurs when index is not found
	ErrIndexNotFound = native_errors.New("index not found")

	// FuncMap for template rendering
	FuncMap = template.FuncMap{}
)

type Service struct {
	random   *rand.Rand
	renderer *renderer.Service
	value    string
	db       db.Service
}

type Config struct {
	Value string
}

func Flags(fs *flag.FlagSet, prefix string) *Config {
	var config Config

	flags.New("Value", "Value key").Prefix(prefix).DocPrefix("search").StringVar(fs, &config.Value, "value", nil)

	return &config
}

func New(config *Config, dbService db.Service, rendererService *renderer.Service) Service {
	return Service{
		value:    config.Value,
		random:   rand.New(rand.NewSource(time.Now().Unix())),
		db:       dbService,
		renderer: rendererService,
	}
}

func (s Service) getCollectionID(ctx context.Context, collection string) (uint64, string, error) {
	collectionID, language, err := s.getCollection(ctx, collection)
	if err != nil {
		return 0, "", fmt.Errorf("get collection: %w", err)
	}
	if collectionID == 0 {
		return 0, "", ErrIndexNotFound
	}
	return collectionID, language, nil
}

func (s Service) HasCollection(ctx context.Context, collection string) bool {
	collectionID, _, err := s.getCollection(ctx, collection)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "check if collection exists", slog.Any("error", err))
	}
	return collectionID != 0
}

func (s Service) GetByID(ctx context.Context, collection, id string) (model.Quote, error) {
	collectionID, language, err := s.getCollectionID(ctx, collection)
	if err != nil {
		return model.Quote{}, err
	}

	return s.getQuote(ctx, collectionID, language, id)
}

func (s Service) Search(ctx context.Context, collection, query, last string) (model.Quote, error) {
	collectionID, language, err := s.getCollectionID(ctx, collection)
	if err != nil {
		return model.Quote{}, err
	}

	return s.searchQuote(ctx, collectionID, language, query, last)
}

func (s Service) Random(ctx context.Context, collection string) (model.Quote, error) {
	collectionID, language, err := s.getCollectionID(ctx, collection)
	if err != nil {
		return model.Quote{}, err
	}

	return s.getRandomQuote(ctx, collectionID, language)
}

func (s Service) TemplateFunc(w http.ResponseWriter, r *http.Request) (renderer.Page, error) {
	return renderer.NewPage("public", http.StatusOK, nil), nil
}
