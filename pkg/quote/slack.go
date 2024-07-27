package quote

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
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

var i18n map[string]string = map[string]string{
	cancelValue: "Annuler",
	nextValue:   "Une autre ?",
	sendValue:   "Envoyer",
	"not_found": "On n'a rien trouvé pour",
	"title":     "Posté par ",
}

type Service struct {
	search      search.Service
	redisClient redis.Client
	tracer      trace.Tracer
	website     string
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
	if !s.search.HasIndex(ctx, payload.Command) {
		return slack.NewEphemeralMessage("unknown command")
	}

	return s.getQuoteBlock(ctx, payload.Command, payload.Text, 0)
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

		return s.getQuoteResponse(action.BlockID, quote, "", payload.User.ID, 0)
	}

	if action.ActionID == nextValue {
		lastIndex := strings.LastIndexAny(action.Value, "@")
		if lastIndex < 1 {
			return slack.NewEphemeralMessage(fmt.Sprintf("button value seems wrong: %s", action.Value))
		}

		offset, err := strconv.Atoi(action.Value[lastIndex+1:])
		if err != nil {
			return slack.NewEphemeralMessage("offset is not numeric")
		}

		return s.getQuoteBlock(ctx, action.BlockID, action.Value[:lastIndex], offset)
	}

	return slack.NewEphemeralMessage("We don't understand what to do.")
}

func (s Service) getQuote(ctx context.Context, index, text string, offset int) (model.Quote, error) {
	quote, err := s.search.Search(ctx, index, strings.TrimSpace(text), offset)
	if err != nil && err == search.ErrNotFound {
		quote, err = s.search.Random(ctx, index)
		if err != nil {
			return model.Quote{}, err
		}
	}

	return quote, err
}

func (s Service) getQuoteBlock(ctx context.Context, index, query string, offset int) slack.Response {
	quote, err := s.getQuote(ctx, index, query, offset)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "search error", slog.String("index", index), slog.String("query", query), slog.Int("offset", offset), slog.Any("error", err))
		return slack.NewError(err)
	}

	return s.getQuoteResponse(index, quote, query, "", offset)
}

func (s Service) getQuoteResponse(index string, quote model.Quote, query, user string, offset int) slack.Response {
	content := s.getContentBlock(index, quote)
	if httpmodel.IsNil(content) {
		return slack.NewEphemeralMessage(fmt.Sprintf("%s `%s`", i18n["not_found"], query))
	}

	if len(user) == 0 {
		if len(query) == 0 {
			query = " "
		}

		return slack.NewEphemeralMessage("").AddBlock(content).AddBlock(slack.NewActions(index, slack.NewButtonElement(i18n[cancelValue], cancelValue, "", "danger"), slack.NewButtonElement(i18n[nextValue], nextValue, fmt.Sprintf("%s@%d", query, offset+1), ""), slack.NewButtonElement(i18n[sendValue], sendValue, quote.ID, "primary")))
	}

	return slack.NewResponse("").WithDeleteOriginal().AddBlock(content).AddBlock(slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("%s <@%s>", i18n["title"], user))))
}

func (s Service) getContentBlock(indexName string, quote model.Quote) slack.Block {
	switch indexName {
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
