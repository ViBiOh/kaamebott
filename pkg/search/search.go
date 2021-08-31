package search

import (
	"context"
	native_errors "errors"
	"flag"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"time"

	"github.com/ViBiOh/httputils/v4/pkg/db"
	"github.com/ViBiOh/httputils/v4/pkg/flags"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
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

// App of package
type App struct {
	dbApp       db.App
	rendererApp renderer.App
	random      *rand.Rand
	value       string
}

// Config of package
type Config struct {
	value *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string) Config {
	return Config{
		value: flags.New(prefix, "search", "Value").Default("value", nil).Label("Value key").ToString(fs),
	}
}

// New creates new App from Config
func New(config Config, dbApp db.App, rendererApp renderer.App) App {
	return App{
		value:       *config.value,
		random:      rand.New(rand.NewSource(time.Now().Unix())),
		dbApp:       dbApp,
		rendererApp: rendererApp,
	}
}

func (a App) getCollectionID(ctx context.Context, collection string) (uint64, error) {
	collectionID, err := a.getCollection(ctx, collection)
	if err != nil {
		return 0, fmt.Errorf("unable to get collection: %s", err)
	}
	if collectionID == 0 {
		return 0, ErrIndexNotFound
	}
	return collectionID, nil
}

// HasCollection determines if defined collection is configured
func (a App) HasCollection(collection string) bool {
	collectionID, err := a.getCollection(context.Background(), collection)
	if err != nil {
		logger.Error("unable to check if collection exists: %s", err)
	}
	return collectionID != 0
}

// GetByID find object by ID
func (a App) GetByID(ctx context.Context, collection, id string) (model.Quote, error) {
	collectionID, err := a.getCollectionID(ctx, collection)
	if err != nil {
		return model.Quote{}, err
	}

	return a.getQuote(ctx, collectionID, id)
}

// Search for a quote
func (a App) Search(ctx context.Context, collection, query, last string) (model.Quote, error) {
	collectionID, err := a.getCollectionID(ctx, collection)
	if err != nil {
		return model.Quote{}, err
	}

	return a.searchQuote(ctx, collectionID, query, last)
}

// Random quote
func (a App) Random(ctx context.Context, collection string) (model.Quote, error) {
	collectionID, err := a.getCollectionID(ctx, collection)
	if err != nil {
		return model.Quote{}, err
	}

	return a.getRandomQuote(ctx, collectionID)
}

// TemplateFunc used for rendering GUI
func (a App) TemplateFunc(w http.ResponseWriter, r *http.Request) (string, int, map[string]interface{}, error) {
	return "public", http.StatusOK, nil, nil
}
