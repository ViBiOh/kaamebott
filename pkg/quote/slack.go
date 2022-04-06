package quote

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
	"github.com/ViBiOh/kaamebott/pkg/slack"
)

const (
	cancelValue = "cancel"
	nextValue   = "next"
	sendValue   = "send"
)

var i18n map[string]map[string]string = map[string]map[string]string{
	"french": {
		cancelValue: "Annuler",
		nextValue:   "Une autre ?",
		sendValue:   "Envoyer",
		"not_found": "On n'a rien trouvé pour",
		"title":     "partage une petite _quote_",
	},
	"english": {
		cancelValue: "Cancel",
		nextValue:   "Another?",
		sendValue:   "Send",
		"not_found": "We didn't find anything for",
		"title":     "shares a _quote_",
	},
}

// App of package
type App struct {
	searchApp search.App
	website   string
}

// New creates new App from Config
func New(website string, searchApp search.App) App {
	return App{
		website:   website,
		searchApp: searchApp,
	}
}

// SlackCommand handler
func (a App) SlackCommand(ctx context.Context, payload slack.InteractivePayload) slack.Response {
	if !a.searchApp.HasCollection(payload.Command) {
		return slack.NewEphemeralMessage("unknown command")
	}

	return a.getQuoteBlock(ctx, payload.Command, payload.Text, "")
}

// SlackInteract handler
func (a App) SlackInteract(ctx context.Context, user string, actions []slack.InteractiveAction) slack.Response {
	action := actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, not now.")
	}

	if action.ActionID == sendValue {
		quote, err := a.searchApp.GetByID(ctx, action.BlockID, action.Value)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("unable to find asked quote: %s", err))
		}

		return a.getQuoteResponse(quote, "", user)
	}

	if action.ActionID == nextValue {
		lastIndex := strings.LastIndexAny(action.Value, "@")
		if lastIndex < 1 {
			return slack.NewEphemeralMessage(fmt.Sprintf("button value seems wrong: %s", action.Value))
		}

		return a.getQuoteBlock(ctx, action.BlockID, action.Value[:lastIndex], action.Value[lastIndex+1:])
	}

	return slack.NewEphemeralMessage("We don't understand what to do.")
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
		return slack.NewEphemeralMessage(fmt.Sprintf("Oh, it's broken 😱. Reason: %s", err))
	}

	return a.getQuoteResponse(quote, text, "")
}

func (a App) getQuoteResponse(quote model.Quote, query, user string) slack.Response {
	content := a.getContentBlock(quote)
	if content == slack.EmptySection {
		return slack.NewEphemeralMessage(fmt.Sprintf("%s `%s`", i18n[quote.Language]["not_now"], query))
	}

	if len(user) == 0 {
		return slack.Response{
			ResponseType:    "ephemeral",
			ReplaceOriginal: true,
			Blocks: []slack.Block{
				content,
				slack.NewActions(quote.Collection, slack.NewButtonElement(i18n[quote.Language][cancelValue], cancelValue, "", "danger"), slack.NewButtonElement(i18n[quote.Language][nextValue], nextValue, fmt.Sprintf("%s@%s", query, quote.ID), ""),
					slack.NewButtonElement(i18n[quote.Language][sendValue], sendValue, quote.ID, "primary")),
			},
		}
	}

	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slack.NewSection(slack.NewText(fmt.Sprintf("<@%s> %s", user, i18n[quote.Language]["title"])), nil),
			content,
		},
	}
}

func (a App) getContentBlock(quote model.Quote) slack.Block {
	switch quote.Collection {
	case "kaamelott":
		return a.getKaamelottBlock(quote)
	case "oss117":
		return a.getOss117Block(quote)
	case "office":
		return a.getOfficeBlock(quote)
	default:
		return slack.EmptySection
	}
}

func (a App) getKaamelottBlock(quote model.Quote) slack.Block {
	return a.getResultBlock(quote, "https://kaamelott-soundboard.2ec0b4.fr/#son/", "kaamelott.png")
}

func (a App) getOss117Block(quote model.Quote) slack.Block {
	return a.getResultBlock(quote, "https://trazip-oss-117-quotes-api.herokuapp.com/api/v1/quotes/", "oss117.png")
}

func (a App) getOfficeBlock(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*%s*\n\n%s", quote.Context, quote.Value))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/office.jpg", a.website), "office")

	return slack.NewSection(text, accessory)
}

func (a App) getResultBlock(quote model.Quote, urlPrefix, imageName string) slack.Block {
	text := slack.NewText(fmt.Sprintf("*<%s%s|%s>*\n\n_%s_ %s", urlPrefix, url.PathEscape(quote.ID), quote.Context, quote.Character, quote.Value))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/%s", a.website, imageName), "oss117")

	return slack.NewSection(text, accessory)
}
