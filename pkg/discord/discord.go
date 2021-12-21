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
	queryParam       = "recherche"
	contentSeparator = "@"
)

var discordRequest = request.New().URL("https://discord.com/api/v8")

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
	var message interactionRequest
	if err := httpjson.Parse(r, &message); err != nil {
		httperror.BadRequest(w, err)
		return
	}

	if message.Type == pingInteraction {
		httpjson.Write(w, http.StatusOK, interactionResponse{Type: pongCallback})
		return
	}

	a.handleQuote(w, r, message)
}

func (a App) handleQuote(w http.ResponseWriter, r *http.Request, webhook interactionRequest) {
	index, err := a.checkRequest(webhook)
	if err != nil {
		respond(w, newEphemeral(false, err.Error()))
		return
	}

	queryValue := a.getQuery(webhook)
	switch strings.Count(queryValue, contentSeparator) {
	case 0:
		respond(w, a.handleSearch(r.Context(), index, queryValue, ""))

	case 1:
		var last string
		lastIndex := strings.LastIndexAny(queryValue, contentSeparator)
		last = queryValue[lastIndex+1:]
		queryValue = queryValue[:lastIndex]
		respond(w, a.handleSearch(r.Context(), index, queryValue, last))

	case 2:
		quote, err := a.searchApp.GetByID(r.Context(), index, strings.Trim(queryValue, contentSeparator))
		if err != nil {
			respond(w, newEphemeral(true, err.Error()))
			return
		}

		respond(w, a.quoteResponse(webhook.Member.User.ID, quote))

	case 3:
		respond(w, newEphemeral(true, "Ok, pas maintenant."))
	}
}

func (a App) checkRequest(message interactionRequest) (string, error) {
	index, ok := indexes[message.Data.Name]
	if !ok {
		return "", fmt.Errorf("unknown command `%s`", message.Data.Name)
	}

	return index, nil
}

func (a App) getQuery(message interactionRequest) string {
	switch message.Type {
	case messageComponentInteraction:
		return message.Data.CustomID
	case applicationCommandInteraction:
		for _, option := range message.Data.Options {
			if strings.EqualFold(option.Name, queryParam) {
				return option.Value
			}
		}
	}

	return ""
}

func (a App) handleSearch(ctx context.Context, index, query, last string) interactionResponse {
	quote, err := a.searchApp.Search(ctx, index, query, last)
	if err != nil && !errors.Is(err, search.ErrNotFound) {
		return newEphemeral(len(last) != 0, fmt.Sprintf("Ah, c'est cassé 😱. La raison : %s", err))
	}

	if len(quote.ID) == 0 {
		return newEphemeral(len(last) != 0, fmt.Sprintf("On n'a rien trouvé pour `%s`", query))
	}

	return a.interactiveResponse(quote, len(last) != 0, query)
}

func (a App) interactiveResponse(quote model.Quote, replace bool, recherche string) interactionResponse {
	webhookType := channelMessageWithSourceCallback
	if replace {
		webhookType = updateMessageCallback
	}

	instance := interactionResponse{Type: webhookType}
	instance.Data.Flags = ephemeralMessage
	instance.Data.Embeds = []embed{a.getQuoteEmbed(quote)}
	instance.Data.Components = []component{
		{
			Type: actionRowType,
			Components: []component{
				newButton(primaryButton, "Envoyer", fmt.Sprintf("%s%s%s", contentSeparator, quote.ID, contentSeparator)),
				newButton(secondaryButton, "Une autre ?", fmt.Sprintf("%s%s%s", recherche, contentSeparator, quote.ID)),
				newButton(dangerButton, "Annuler", fmt.Sprintf("%s%s%s", contentSeparator, contentSeparator, contentSeparator)),
			},
		},
	}

	return instance
}

func (a App) quoteResponse(user string, quote model.Quote) interactionResponse {
	instance := interactionResponse{Type: channelMessageWithSourceCallback}
	instance.Data.Content = fmt.Sprintf("<@!%s> vous partage une petite quote", user)
	instance.Data.AllowedMentions = allowedMention{
		Parse: []string{},
	}
	instance.Data.Embeds = []embed{a.getQuoteEmbed(quote)}

	return instance
}

func (a App) getQuoteEmbed(quote model.Quote) embed {
	switch quote.Collection {
	case kaamelottIndexName:
		return a.getKaamelottEmbeds(quote)
	default:
		return embed{
			Title:       "Error",
			Description: fmt.Sprintf("unable to render quote of collection `%s`", quote.Collection),
		}
	}
}

func respond(w http.ResponseWriter, response interactionResponse) {
	httpjson.Write(w, http.StatusOK, response)
}
