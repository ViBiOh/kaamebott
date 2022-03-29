package quote

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
	"github.com/ViBiOh/kaamebott/pkg/slack"
)

const (
	cancelValue = "cancel"
	nextValue   = "next"
	sendValue   = "send"
)

var cancelButton = slack.NewButtonElement("Annuler", cancelValue, "", "danger")

// Config of package
type Config struct {
	website *string
}

// App of package
type App struct {
	searchApp search.App
	website   string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		website: flags.String(fs, prefix, "slack", "Website", "URL of public website", "https://kaamebott.vibioh.fr", overrides),
	}
}

// New creates new App from Config
func New(config Config, searchApp search.App) App {
	return App{
		website:   strings.TrimSpace(*config.website),
		searchApp: searchApp,
	}
}

// SlackCommand handler
func (a App) SlackCommand(w http.ResponseWriter, r *http.Request, pathName, text string) {
	if !a.searchApp.HasCollection(pathName) {
		httperror.NotFound(w)
	}

	httpjson.Write(w, http.StatusOK, a.getQuoteBlock(r.Context(), pathName, r.FormValue("text"), ""))
}

// SlackInteract handler
func (a App) SlackInteract(r *http.Request, user string, actions []slack.InteractiveAction) slack.Response {
	action := actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, pas maintenant.")
	}

	ctx := context.Background()

	if action.ActionID == sendValue {
		quote, err := a.searchApp.GetByID(ctx, action.BlockID, action.Value)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("Impossible de retrouver la citation demandée: %s", err))
		}

		return a.getQuoteResponse(quote, "", user)
	}

	if action.ActionID == nextValue {
		lastIndex := strings.LastIndexAny(action.Value, "@")
		if lastIndex < 1 {
			return slack.NewEphemeralMessage(fmt.Sprintf("La valeur du bouton semble incorrecte: %s", action.Value))
		}

		return a.getQuoteBlock(ctx, action.BlockID, action.Value[:lastIndex], action.Value[lastIndex+1:])
	}

	return slack.NewEphemeralMessage("On ne comprend pas l'action à effectuer")
}

func (a App) getQuote(ctx context.Context, index, text string, last string) (model.Quote, error) {
	quote, err := a.searchApp.Search(ctx, index, text, last)
	if err != nil && err == search.ErrNotFound {
		quote, err = a.searchApp.Random(ctx, index)
		if err != nil {
			return model.Quote{}, err
		}
	}

	return quote, err
}

func (a App) getQuoteBlock(ctx context.Context, index, text string, last string) slack.Response {
	quote, err := a.getQuote(ctx, index, text, last)
	if err != nil {
		return slack.NewEphemeralMessage(fmt.Sprintf("Ah, c'est cassé 😱. La raison : %s", err))
	}

	return a.getQuoteResponse(quote, text, "")
}

func (a App) getQuoteResponse(quote model.Quote, query, user string) slack.Response {
	content := a.getContentBlock(quote)
	if content == slack.EmptySection {
		return slack.NewEphemeralMessage(fmt.Sprintf("On n'a rien trouvé pour `%s`", query))
	}

	if len(user) == 0 {
		return slack.Response{
			ResponseType:    "ephemeral",
			ReplaceOriginal: true,
			Blocks: []slack.Block{
				content,
				slack.NewActions(quote.Collection, cancelButton, slack.NewButtonElement("Une autre ?", nextValue, fmt.Sprintf("%s@%s", query, quote.ID), ""),
					slack.NewButtonElement("Envoyer", sendValue, quote.ID, "primary")),
			},
		}
	}

	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slack.NewSection(slack.NewText(fmt.Sprintf("<@%s> vous partage une petite _quote_  ", user)), nil),
			content,
		},
	}
}

func (a App) getContentBlock(quote model.Quote) slack.Block {
	switch quote.Collection {
	case "kaamelott":
		return a.getKaamelottBlock(quote)
	default:
		return slack.EmptySection
	}
}

func (a App) getKaamelottBlock(output model.Quote) slack.Block {
	titleLink := fmt.Sprintf("https://kaamelott-soundboard.2ec0b4.fr/#son/%s", url.PathEscape(output.ID))
	content := fmt.Sprintf("_%s_ %s", output.Character, output.Value)

	text := slack.NewText(fmt.Sprintf("*<%s|%s>*\n\n%s", titleLink, output.Context, content))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/kaamelott.png", a.website), "kaamelott")

	return slack.NewSection(text, accessory)
}
