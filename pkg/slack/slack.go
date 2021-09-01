package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ViBiOh/httputils/v4/pkg/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

const (
	commandQuote = "quote"
)

// Config of package
type Config struct {
	clientID      *string
	clientSecret  *string
	signingSecret *string
	website       *string
}

// App of package
type App struct {
	searchApp     search.App
	clientID      string
	clientSecret  string
	website       string
	signingSecret []byte
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string) Config {
	return Config{
		clientID:      flags.New(prefix, "slack", "ClientID").Default("", nil).Label("ClientID").ToString(fs),
		clientSecret:  flags.New(prefix, "slack", "ClientSecret").Default("", nil).Label("ClientSecret").ToString(fs),
		signingSecret: flags.New(prefix, "slack", "SigningSecret").Default("", nil).Label("Signing secret").ToString(fs),
		website:       flags.New(prefix, "slack", "Website").Default("https://kaamebott.vibioh.fr", nil).Label("URL of public website").ToString(fs),
	}
}

// New creates new App from Config
func New(config Config, searchApp search.App) *App {
	return &App{
		clientID:      *config.clientID,
		clientSecret:  *config.clientSecret,
		signingSecret: []byte(*config.signingSecret),
		website:       *config.website,
		searchApp:     searchApp,
	}
}

// Handler for net/http
func (a App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.checkSignature(r) {
			httperror.Unauthorized(w, errors.New("invalid signature"))
			return
		}

		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusOK)
			return

		case http.MethodPost:
			if r.URL.Path == "/interactive" {
				a.handleInteract(w, r)
				return
			} else if r.URL.Path == "/oauth" {
				a.handleOauth(w, r)
				return
			}

			pathName := strings.TrimPrefix(r.URL.Path, "/")
			if a.searchApp.HasCollection(pathName) {
				a.handleCommand(w, r, commandQuote, pathName)
				return
			}

			httperror.NotFound(w)
			return

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func (a App) checkSignature(r *http.Request) bool {
	tsValue, err := strconv.ParseInt(r.Header.Get("X-Slack-Request-Timestamp"), 10, 64)
	if err != nil {
		logger.Error("unable to parse timestamp: %s", err)
		return false
	}

	if time.Unix(tsValue, 0).Before(time.Now().Add(time.Minute * -5)) {
		logger.Warn("timestamp is from 5 minutes ago")
		return false
	}

	body, err := request.ReadBodyRequest(r)
	if err != nil {
		logger.Warn("unable to read request body: %s", err)
		return false
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	slackSignature := r.Header.Get("X-Slack-Signature")
	signatureValue := []byte(fmt.Sprintf("v0:%d:%s", tsValue, body))

	sig := hmac.New(sha256.New, a.signingSecret)
	sig.Write(signatureValue)
	ownSignature := fmt.Sprintf("v0=%s", hex.EncodeToString(sig.Sum(nil)))

	if hmac.Equal([]byte(slackSignature), []byte(ownSignature)) {
		return true
	}

	logger.Error("signature mismatch from slack's one: `%s`", slackSignature)
	return false
}

func (a App) handleCommand(w http.ResponseWriter, r *http.Request, commandName, collectionName string) {
	switch commandName {
	case commandQuote:
		a.handleQuote(w, r, collectionName)
	default:
		a.returnEphemeral(w, "On ne comprend pas très bien ce que vous attendez de nous... 🧐")
	}
}

func (a App) returnEphemeral(w http.ResponseWriter, message string) {
	httpjson.Write(w, http.StatusOK, model.NewEphemeralMessage(message))
}

func (a App) handleInteract(w http.ResponseWriter, r *http.Request) {
	rawPayload := r.FormValue("payload")
	var payload model.Interactive

	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		a.returnEphemeral(w, fmt.Sprintf("cannot unmarshall payload: %v", err))
		return
	}

	a.send(payload.ResponseURL, a.handleQuoteInteract(r, payload.User.ID, payload.Actions))
}

func (a App) send(url string, message model.Response) {
	_, err := request.New().Post(url).JSON(context.Background(), message)
	if err != nil {
		logger.Error("unable to send response: %s", err)
	}
}
