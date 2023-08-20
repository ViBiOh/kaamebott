package quote

import (
	"context"
	"fmt"
	"strings"

	"github.com/ViBiOh/ChatPotte/slack"
	httpmodel "github.com/ViBiOh/httputils/v4/pkg/model"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
	"go.opentelemetry.io/otel/trace"
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
		"title":     "Posté par ",
	},
	"english": {
		cancelValue: "Cancel",
		nextValue:   "Another?",
		sendValue:   "Send",
		"not_found": "We didn't find anything for",
		"title":     "Posted by ",
	},
}

// App of package
type App struct {
	searchApp   search.App
	redisClient redis.Client
	tracer      trace.Tracer
	website     string
}

// New creates new App from Config
func New(website string, searchApp search.App, redisApp redis.Client, tracerProvider trace.TracerProvider) App {
	app := App{
		website:     website,
		searchApp:   searchApp,
		redisClient: redisApp,
	}

	if tracerProvider != nil {
		app.tracer = tracerProvider.Tracer("quote")
	}

	return app
}

// SlackCommand handler
func (a App) SlackCommand(ctx context.Context, payload slack.SlashPayload) slack.Response {
	if !a.searchApp.HasCollection(payload.Command) {
		return slack.NewEphemeralMessage("unknown command")
	}

	return a.getQuoteBlock(ctx, payload.Command, payload.Text, "")
}

// SlackInteract handler
func (a App) SlackInteract(ctx context.Context, payload slack.InteractivePayload) slack.Response {
	if len(payload.Actions) == 0 {
		return slack.NewEphemeralMessage("No action provided")
	}

	action := payload.Actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, not now.")
	}

	if action.ActionID == sendValue {
		quote, err := a.searchApp.GetByID(ctx, action.BlockID, action.Value)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("find asked quote: %s", err))
		}

		return a.getQuoteResponse(quote, "", payload.User.ID)
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
	quote, err := a.searchApp.Search(ctx, index, strings.TrimSpace(text), last)
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
		return slack.NewError(err)
	}

	return a.getQuoteResponse(quote, text, "")
}

func (a App) getQuoteResponse(quote model.Quote, query, user string) slack.Response {
	content := a.getContentBlock(quote)
	if httpmodel.IsNil(content) {
		if len(quote.Language) == 0 {
			quote.Language = "english"
		}
		return slack.NewEphemeralMessage(fmt.Sprintf("%s `%s`", i18n[quote.Language]["not_found"], query))
	}

	if len(user) == 0 {
		if len(query) == 0 {
			query = " "
		}

		return slack.NewEphemeralMessage("").AddBlock(content).AddBlock(slack.NewActions(quote.Collection, slack.NewButtonElement(i18n[quote.Language][cancelValue], cancelValue, "", "danger"), slack.NewButtonElement(i18n[quote.Language][nextValue], nextValue, fmt.Sprintf("%s@%s", query, quote.ID), ""), slack.NewButtonElement(i18n[quote.Language][sendValue], sendValue, quote.ID, "primary")))
	}

	return slack.NewResponse("").WithDeleteOriginal().AddBlock(content).AddBlock(slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("%s <@%s>", i18n[quote.Language]["title"], user))))
}

func (a App) getContentBlock(quote model.Quote) slack.Block {
	switch quote.Collection {
	case "kaamelott":
		return a.getKaamelottBlock(quote)
	case "kaamelott_gif":
		return a.getKaamelottGifBlock(quote)
	case "oss117":
		return a.getOss117Block(quote)
	case "office":
		return a.getOfficeBlock(quote)
	case "films":
		return a.getFilmBlock(quote)
	case "telerealite":
		return a.getTelerealiteBlock(quote)
	default:
		return nil
	}
}

func (a App) getKaamelottBlock(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*<%s|%s>*\n\n_%s_ %s", quote.URL, quote.Context, quote.Character, quote.Value))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/kaamelott.png", a.website), "kaamelott")

	return slack.NewSection(text).WithAccessory(accessory)
}

func (a App) getKaamelottGifBlock(quote model.Quote) slack.Block {
	return slack.NewAccessory(quote.URL, quote.Value)
}

func (a App) getOss117Block(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*%s*\n\n_%s_ %s", quote.Context, quote.Character, quote.Value))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/oss117.png", a.website), "oss117")

	return slack.NewSection(text).WithAccessory(accessory)
}

func (a App) getOfficeBlock(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*%s*\n\n%s", quote.Context, quote.Value))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/office.jpg", a.website), "office")

	return slack.NewSection(text).WithAccessory(accessory)
}

func (a App) getFilmBlock(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*%s*\n\n_%s_\n%s", quote.Context, quote.Character, quote.Value))

	return slack.NewSection(text)
}

func (a App) getTelerealiteBlock(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*%s*\n\n%s", quote.Character, quote.Value))

	return slack.NewSection(text)
}
