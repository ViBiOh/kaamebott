package quote

import (
	"context"
	"errors"
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

func (s Service) getQuoteBlock(ctx context.Context, index, query string, offset int) slack.Response {
	quote, err := s.search.Search(ctx, index, strings.TrimSpace(query), offset)
	if err != nil {
		if errors.Is(err, search.ErrNotFound) {
			return slack.NewEphemeralMessage(fmt.Sprintf("We found nothing for `%s`", query))
		}

		if errors.Is(err, search.ErrIndexNotFound) {
			return slack.NewEphemeralMessage("Tout doux bijou, le moteur de recherche était pété, je le redémarre.")
		}

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

		response := slack.NewEphemeralMessage("")
		for _, block := range content {
			response = response.AddBlock(block)
		}

		return response.AddBlock(slack.NewActions(index, slack.NewButtonElement(i18n[cancelValue], cancelValue, "", "danger"), slack.NewButtonElement(i18n[nextValue], nextValue, fmt.Sprintf("%s@%d", query, offset+1), ""), slack.NewButtonElement(i18n[sendValue], sendValue, quote.ID, "primary")))
	}

	response := slack.NewResponse("").WithDeleteOriginal()
	for _, block := range content {
		response = response.AddBlock(block)
	}

	return response.AddBlock(slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("%s <@%s>", i18n["title"], user))))
}

func (s Service) getContentBlock(indexName string, quote model.Quote) []slack.Block {
	switch indexName {
	case "kaamelott":
		return s.getKaamelottBlock(quote)

	case "oss117":
		return []slack.Block{s.getOss117Block(quote)}

	case "abitbol":
		return []slack.Block{s.getAbitbolBlock(quote)}

	default:
		return nil
	}
}

func (s Service) getKaamelottBlock(quote model.Quote) []slack.Block {
	var text string

	if len(quote.URL) != 0 && len(quote.Context) != 0 {
		text = fmt.Sprintf("*<%s|%s>*", quote.URL, quote.Context)
	}

	if len(quote.Character) != 0 {
		if len(text) != 0 {
			text += "\n\n"
		}

		text += fmt.Sprintf("_%s_", quote.Character)
	}

	if len(quote.Value) != 0 {
		if len(text) != 0 {
			text += " "
		}

		text += quote.Value
	}

	section := slack.NewSection(slack.NewText(text))
	var sections []slack.Block

	if len(quote.Image) == 0 {
		section = section.WithAccessory(slack.NewAccessory(fmt.Sprintf("%s/images/kaamelott.png", s.website), "kaamelott"))
		sections = append(sections, section)
	} else {
		sections = append(sections, section)
		sections = append(sections, slack.NewImage(quote.Image, quote.Value, quote.Character))
	}

	return sections
}

func (s Service) getOss117Block(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*%s*\n\n_%s_ %s", quote.Context, quote.Character, quote.Value))
	accessory := slack.NewAccessory(fmt.Sprintf("%s/images/oss117.png", s.website), "oss117")

	return slack.NewSection(text).WithAccessory(accessory)
}

func (s Service) getAbitbolBlock(quote model.Quote) slack.Block {
	text := slack.NewText(fmt.Sprintf("*<%s|%s>*\n\n_%s_ %s", quote.URL, quote.Context, quote.Character, quote.Value))
	accessory := slack.NewAccessory(quote.Image, "abitbol")

	return slack.NewSection(text).WithAccessory(accessory)
}
