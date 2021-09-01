package discord

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/query"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

const (
	queryParam = "recherche"
)

var (
	discordRequest = request.New().URL("https://discord.com/api/v8")
)

// App of package
type App struct {
	searchApp search.App

	applicationID string
	clientID      string
	clientSecret  string
	website       string
	publicKey     []byte
}

// Config of package
type Config struct {
	applicationID *string
	publicKey     *string
	clientID      *string
	clientSecret  *string
	website       *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		applicationID: flags.New(prefix, "discord", "ApplicationID").Default("", overrides).Label("Application ID").ToString(fs),
		publicKey:     flags.New(prefix, "discord", "PublicKey").Default("", overrides).Label("Public Key").ToString(fs),
		clientID:      flags.New(prefix, "discord", "ClientID").Default("", overrides).Label("Client ID").ToString(fs),
		clientSecret:  flags.New(prefix, "discord", "ClientSecret").Default("", overrides).Label("Client Secret").ToString(fs),
		website:       flags.New(prefix, "discord", "Website").Default("https://kaamebott.vibioh.fr", overrides).Label("URL of public website").ToString(fs),
	}
}

// New creates new App from Config
func New(config Config, searchApp search.App) (App, error) {
	publicKeyStr := *config.publicKey
	if len(publicKeyStr) == 0 {
		return App{}, nil
	}

	publicKey, err := hex.DecodeString(publicKeyStr)
	if err != nil {
		return App{}, fmt.Errorf("unable to decode public key string: %s", err)
	}

	return App{
		applicationID: *config.applicationID,
		publicKey:     publicKey,
		clientID:      *config.clientID,
		clientSecret:  *config.clientSecret,
		website:       *config.website,
		searchApp:     searchApp,
	}, nil
}

// Handler for request. Should be use with net/http
func (a App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth" {
			a.handleOauth(w, r)
			return
		}

		if !a.checkSignature(r) {
			httperror.Unauthorized(w, errors.New("invalid signature"))
			return
		}

		if query.IsRoot(r) && r.Method == http.MethodPost {
			a.handleWebhook(w, r)
			return
		}

		httperror.NotFound(w)
	})
}

func (a App) checkSignature(r *http.Request) bool {
	sig, err := hex.DecodeString(r.Header.Get("X-Signature-Ed25519"))
	if err != nil {
		logger.Warn("unable to decode signature string: %s", err)
		return false
	}

	if len(sig) != ed25519.SignatureSize || sig[63]&224 != 0 {
		logger.Warn("length of signature is invalid: %d", len(sig))
		return false
	}

	body, err := request.ReadBodyRequest(r)
	if err != nil {
		logger.Warn("unable to read request body: %s", err)
		return false
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var msg bytes.Buffer
	msg.WriteString(r.Header.Get("X-Signature-Timestamp"))
	msg.Write(body)

	return ed25519.Verify(ed25519.PublicKey(a.publicKey), msg.Bytes(), sig)
}

func (a App) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var webhookMessage webhook
	if err := httpjson.Parse(r, &webhookMessage); err != nil {
		httperror.BadRequest(w, err)
		return
	}

	if webhookMessage.Type == 1 {
		httpjson.Write(w, http.StatusOK, webhook{Type: 1})
		return
	}

	a.handleQuote(w, r, webhookMessage)
}

func (a App) handleQuote(w http.ResponseWriter, r *http.Request, webhookMessage webhook) {
	index, queryValue, err := a.parseQuoteRequest(webhookMessage)
	if err != nil {
		httpjson.Write(w, http.StatusOK, errorResponse(err))
		return
	}

	quote, recherche, err := a.findQuote(r.Context(), index, queryValue)
	if err != nil {
		httperror.InternalServerError(w, err)
		return
	}

	if len(quote.ID) == 0 {
		logger.WithField("command", webhookMessage.Data.Name).WithField("query", queryValue).Warn("No quote found")
		httpjson.Write(w, http.StatusOK, errorResponse(fmt.Errorf("On n'a rien trouvé pour `%s`", queryValue)))
		return
	}

	httpjson.Write(w, http.StatusOK, a.quoteResponse(quote, recherche))
}

func (a App) parseQuoteRequest(webhookMessage webhook) (string, string, error) {
	index, ok := indexes[webhookMessage.Data.Name]
	if !ok {
		return "", "", fmt.Errorf("unknown command `%s`", webhookMessage.Data.Name)
	}

	queryValue := ""
	for _, option := range webhookMessage.Data.Options {
		if strings.EqualFold(option.Name, queryParam) {
			queryValue = strings.TrimSpace(option.Value)
			break
		}
	}

	return index, queryValue, nil
}

func (a App) findQuote(ctx context.Context, index, query string) (model.Quote, string, error) {
	recherche := fmt.Sprintf(`"%s"`, query)

	quote, err := a.searchApp.Search(ctx, index, query, "")
	if err != nil && err != search.ErrNotFound {
		return model.Quote{}, recherche, fmt.Errorf("unable to search quote: %s", err)
	}

	if len(quote.ID) == 0 {
		recherche += " ➡️ aléatoire"
		quote, err = a.searchApp.Random(ctx, index)
		if err != nil {
			return model.Quote{}, recherche, fmt.Errorf("unable to get random quote: %s", err)
		}
	}

	return quote, recherche, err
}

func (a App) quoteResponse(quote model.Quote, recherche string) webhook {
	output := webhook{
		Type: 4, // https://discord.com/developers/docs/interactions/slash-commands#interaction-response-interactionresponsetype
	}

	switch quote.Collection {
	case kaamelottIndexName:
		output.Data.Embeds = []embed{a.getKaamelottEmbeds(quote)}
	default:
		return errorResponse(errors.New("unable to render quote for a human"))
	}

	output.Data.Embeds[0].Fields = append(output.Data.Embeds[0].Fields, newField("Recherche", recherche))

	return output
}

func errorResponse(err error) webhook {
	return webhook{
		Type: 4, // https://discord.com/developers/docs/interactions/slash-commands#interaction-response-interactionresponsetype
		Data: data{
			Content: err.Error(),
			Flags:   64,
		},
	}
}
