package quote

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/httputils/v4/pkg/telemetry"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
	"github.com/ViBiOh/kaamebott/pkg/version"
)

const (
	queryParam = "recherche"

	kaamelottName = "kaamelott"
	oss117Name    = "oss117"
)

var (
	cachePrefix  = version.Redis("discord")
	cancelAction = fmt.Sprintf("action=%s", url.QueryEscape(cancelValue))
)

var indexes = map[string]string{
	kaamelottName: kaamelottName,
	oss117Name:    oss117Name,
}

func (s Service) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, func(context.Context) discord.InteractionResponse) {
	var err error

	ctx, end := telemetry.StartSpan(ctx, s.tracer, "DiscordHandler")
	defer end(&err)

	index, err := s.checkRequest(webhook)
	if err != nil {
		return discord.NewEphemeral(false, err.Error()), nil
	}

	action, query, next, err := s.getQuery(ctx, webhook)
	if err != nil {
		return discord.NewEphemeral(false, err.Error()), nil
	}

	switch action {
	case nextValue:
		return s.handleSearch(ctx, index, query, next), nil

	case sendValue:
		quote, err := s.search.GetByID(ctx, index, query)
		if err != nil {
			slog.LogAttrs(ctx, slog.LevelError, "get by id", slog.String("index", index), slog.String("query", query), slog.Any("error", err))
			return discord.NewError(true, err), nil
		}

		return s.quoteResponse(webhook.Member.User.ID, quote), nil

	case cancelValue:
		return discord.NewEphemeral(true, "Ok, not now."), nil

	default:
		return s.handleSearch(ctx, index, query, ""), nil
	}
}

func (s Service) checkRequest(webhook discord.InteractionRequest) (string, error) {
	var command string
	switch webhook.Type {
	case discord.MessageComponentInteraction:
		command = webhook.Message.Interaction.Name
	case discord.ApplicationCommandInteraction:
		command = webhook.Data.Name
	}

	index, ok := indexes[command]
	if !ok {
		return "", fmt.Errorf("unknown command `%s`", command)
	}

	return index, nil
}

func (s Service) getQuery(ctx context.Context, webhook discord.InteractionRequest) (string, string, string, error) {
	switch webhook.Type {
	case discord.MessageComponentInteraction:

		values, err := discord.RestoreCustomID(ctx, s.redisClient, cachePrefix, webhook.Data.CustomID, []string{cancelAction})
		if err != nil {
			return "", "", "", fmt.Errorf("restore id: %w", err)
		}

		switch values.Get("action") {
		case sendValue:
			return sendValue, values.Get("id"), "", nil
		case nextValue:
			return nextValue, values.Get("recherche"), values.Get("id"), nil
		case cancelValue:
			return cancelValue, "", "", nil
		}

	case discord.ApplicationCommandInteraction:
		for _, option := range webhook.Data.Options {
			if strings.EqualFold(option.Name, queryParam) {
				return nextValue, option.Value, "", nil
			}
		}
	}

	return "", "", "", nil
}

func (s Service) handleSearch(ctx context.Context, index, query, last string) discord.InteractionResponse {
	quote, err := s.search.Search(ctx, index, query, last)
	if err != nil && !errors.Is(err, search.ErrNotFound) {
		slog.LogAttrs(ctx, slog.LevelError, "search", slog.String("index", index), slog.String("query", query), slog.String("last", last), slog.Any("error", err))
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("Oh, it's broken 😱. Reason: %s", err))
	}

	if len(quote.ID) == 0 {
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("We found nothing for `%s`", query))
	}

	return s.interactiveResponse(ctx, quote, len(last) != 0, query)
}

func (s Service) interactiveResponse(ctx context.Context, quote model.Quote, replace bool, recherche string) discord.InteractionResponse {
	var err error

	ctx, end := telemetry.StartSpan(ctx, s.tracer, "interactiveResponse")
	defer end(&err)

	webhookType := discord.ChannelMessageWithSource
	if replace {
		webhookType = discord.UpdateMessageCallback
	}

	sendValues := url.Values{}
	sendValues.Add("action", sendValue)
	sendValues.Add("id", quote.ID)

	sendKey, err := discord.SaveCustomID(ctx, s.redisClient, cachePrefix, sendValues)
	if err != nil {
		return discord.NewError(replace, err)
	}

	nextValues := url.Values{}
	nextValues.Add("action", nextValue)
	nextValues.Add("id", quote.ID)
	nextValues.Add("recherche", recherche)

	nextKey, err := discord.SaveCustomID(ctx, s.redisClient, cachePrefix, nextValues)
	if err != nil {
		return discord.NewError(replace, err)
	}

	return discord.NewResponse(webhookType, "").Ephemeral().AddEmbed(s.getQuoteEmbed(quote)).AddComponent(
		discord.Component{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, i18n[quote.Language][sendValue], sendKey),
				discord.NewButton(discord.SecondaryButton, i18n[quote.Language][nextValue], nextKey),
				discord.NewButton(discord.DangerButton, i18n[quote.Language][cancelValue], cancelAction),
			},
		})
}

func (s Service) quoteResponse(user string, quote model.Quote) discord.InteractionResponse {
	return discord.NewResponse(discord.ChannelMessageWithSource, fmt.Sprintf("<@!%s> %s", user, i18n[quote.Language]["title"])).AddEmbed(s.getQuoteEmbed(quote))
}

func (s Service) getQuoteEmbed(quote model.Quote) discord.Embed {
	switch quote.Collection {
	case kaamelottName:
		return s.getKaamelottEmbeds(quote)

	case oss117Name:
		return s.getOss117Embeds(quote)

	default:
		return discord.Embed{
			Title:       "Error",
			Description: fmt.Sprintf("render quote of collection `%s`", quote.Collection),
		}
	}
}

func (s Service) getKaamelottEmbeds(quote model.Quote) discord.Embed {
	var thumbnail, image *discord.Image
	if len(quote.Image) != 0 {
		image = discord.NewImage(quote.Image)
	} else {
		thumbnail = discord.NewImage(fmt.Sprintf("%s/images/kaamelott.png", s.website))
	}

	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		URL:         quote.URL,
		Image:       image,
		Thumbnail:   thumbnail,
		Fields: []discord.Field{
			discord.NewField("Personnage", quote.Character),
		},
	}
}

func (s Service) getOss117Embeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		Thumbnail:   discord.NewImage(fmt.Sprintf("%s/images/oss117.png", s.website)),
		Fields: []discord.Field{
			discord.NewField("Personnage", quote.Character),
		},
	}
}
