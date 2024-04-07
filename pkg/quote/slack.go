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

type Service struct {
	redisClient redis.Client
	tracer      trace.Tracer
	website     string
	search      search.Service
}

func New(website string, searchService search.Service, redisClient redis.Client, tracerProvider trace.TracerProvider) Service {
	service := Service{
		website:     website,
		search:      searchService,
		redisClient: redisClient,
	}

	if tracerProvider != nil {
		service.tracer = tracerProvider.Tracer("quote")
	}

	return service
}

func (s Service) SlackCommand(ctx context.Context, payload slack.SlashPayload) slack.Response {
	if !s.search.HasCollection(ctx, payload.Command) {
		return slack.NewEphemeralMessage("unknown command")
	}

	return s.getQuoteBlock(ctx, payload.Command, payload.Text, "")
}

func (s Service) SlackInteract(ctx context.Context, payload slack.InteractivePayload) slack.Response {
	if len(payload.Actions) == 0 {
		return slack.NewEphemeralMessage("No action provided")
	}

	action := payload.Actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, not now.")
	}

	if action.ActionID == sendValue {
		quote, err := s.search.GetByID(ctx, action.BlockID, action.Value)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("find asked quote: %s", err))
		}

		return s.getQuoteResponse(quote, "", payload.User.ID)
	}

	if action.ActionID == nextValue {
		lastIndex := strings.LastIndexAny(action.Value, "@")
		if lastIndex < 1 {
			return slack.NewEphemeralMessage(fmt.Sprintf("button value seems wrong: %s", action.Value))
		}

		return s.getQuoteBlock(ctx, action.BlockID, action.Value[:lastIndex], action.Value[lastIndex+1:])
	}

	return slack.NewEphemeralMessage("We don't understand what to do.")
}

func (s Service) getQuote(ctx context.Context, index, text string, last string) (model.Quote, error) {
	quote, err := s.search.Search(ctx, index, strings.TrimSpace(text), last)
	if err != nil && err == search.ErrNotFound {
		quote, err = s.search.Random(ctx, index)
		if err != nil {
			return model.Quote{}, err
		}
	}

	return quote, err
}

func (s Service) getQuoteBlock(ctx context.Context, index, text string, last string) slack.Response {
	quote, err := s.getQuote(ctx, index, text, last)
	if err != nil {
		return slack.NewError(err)
	}

	return s.getQuoteResponse(quote, text, "")
}

func (s Service) getQuoteResponse(quote model.Quote, query, user string) slack.Response {
	content := s.getContentBlock(quote)
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

func (s Service) getContentBlock(quote model.Quote) slack.Block {
	switch quote.Collection {
	case "kaamelott":
		return s.getKaamelottBlock(quote)

	case "oss117":
		return s.getOss117Block(quote)

	default:
		return nil
	}
}

func (s Service) getKaamelottBlock(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*<%s|%s>*\n\n_%s_ %s", quote.URL, quote.Context, quote.Character, quote.Value))

	var accessory *slack.Accessory

	if len(quote.Image) != 0 {
		accessory = slack.NewAccessory(quote.Image, quote.Value)
	} else {
		accessory = slack.NewAccessory(fmt.Sprintf("%s/images/kaamelott.png", s.website), "kaamelott")
	}

	return slack.NewSection(text).WithAccessory(accessory)
}

func (s Service) getOss117Block(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*%s*\n\n_%s_ %s", quote.Context, quote.Character, quote.Value))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/oss117.png", s.website), "oss117")

	return slack.NewSection(text).WithAccessory(accessory)
}
