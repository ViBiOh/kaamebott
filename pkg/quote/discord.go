package quote

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
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
	abitbolName   = "abitbol"
)

var (
	cachePrefix  = version.Redis("discord")
	cancelAction = fmt.Sprintf("action=%s", url.QueryEscape(cancelValue))
)

var indexes = map[string]string{
	kaamelottName: kaamelottName,
	oss117Name:    oss117Name,
	abitbolName:   abitbolName,
}

func (s Service) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, bool, func(context.Context) discord.InteractionResponse) {
	var err error

	ctx, end := telemetry.StartSpan(ctx, s.tracer, "DiscordHandler")
	defer end(&err)

	index, err := s.checkRequest(webhook)
	if err != nil {
		return discord.NewEphemeral(false, err.Error()), false, nil
	}

	action, query, offset, err := s.getQuery(ctx, webhook)
	if err != nil {
		return discord.NewEphemeral(false, err.Error()), false, nil
	}

	switch action {
	case nextValue:
		return s.handleSearch(ctx, index, query, offset), false, nil

	case sendValue:
		quote, err := s.search.GetByID(ctx, index, query)
		if err != nil {
			slog.LogAttrs(ctx, slog.LevelError, "get by id", slog.String("index", index), slog.String("query", query), slog.Any("error", err))
			return discord.NewError(true, err), false, nil
		}

		return s.quoteResponse(webhook.Member.User.ID, index, quote)

	case cancelValue:
		return discord.NewEphemeral(true, "Ok, not now."), true, nil

	default:
		return s.handleSearch(ctx, index, query, 0), false, nil
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

func (s Service) getQuery(ctx context.Context, webhook discord.InteractionRequest) (string, string, int, error) {
	switch webhook.Type {
	case discord.MessageComponentInteraction:
		values, err := discord.RestoreCustomID(ctx, s.redisClient, cachePrefix, webhook.Data.CustomID, []string{cancelAction})
		if err != nil {
			return "", "", 0, fmt.Errorf("restore id: %w", err)
		}

		switch values.Get("action") {
		case sendValue:
			return sendValue, values.Get("id"), 0, nil

		case nextValue:
			offset, err := strconv.Atoi(values.Get("offset"))
			if err != nil {
				return "", "", 0, fmt.Errorf("offset is not numeric: %w", err)
			}

			return nextValue, values.Get("search"), offset, nil

		case cancelValue:
			return cancelValue, "", 0, nil
		}

	case discord.ApplicationCommandInteraction:
		for _, option := range webhook.Data.Options {
			if strings.EqualFold(option.Name, queryParam) {
				return nextValue, option.Value, 0, nil
			}
		}
	}

	return "", "", 0, nil
}

func (s Service) handleSearch(ctx context.Context, indexName, query string, offset int) discord.InteractionResponse {
	quote, err := s.search.Search(ctx, indexName, query, offset)

	if err != nil && !errors.Is(err, search.ErrNotFound) {
		if errors.Is(err, search.ErrIndexNotFound) {
			return discord.NewEphemeral(offset != 0, "Tout doux bijou, le moteur de recherche était pété, je le redémarre.")
		}

		slog.LogAttrs(ctx, slog.LevelError, "search", slog.String("index", indexName), slog.String("query", query), slog.Int("offset", offset), slog.Any("error", err))
		return discord.NewEphemeral(offset != 0, fmt.Sprintf("Oh, it's broken 😱. Reason: %s", err))
	}

	if len(quote.ID) == 0 {
		return discord.NewEphemeral(offset != 0, fmt.Sprintf("We found nothing for `%s`", query))
	}

	return s.interactiveResponse(ctx, indexName, quote, query, offset)
}

func (s Service) interactiveResponse(ctx context.Context, indexName string, quote model.Quote, search string, offset int) discord.InteractionResponse {
	var err error

	ctx, end := telemetry.StartSpan(ctx, s.tracer, "interactiveResponse")
	defer end(&err)

	replace := offset != 0

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
	nextValues.Add("offset", strconv.Itoa(offset+1))
	nextValues.Add("search", search)

	nextKey, err := discord.SaveCustomID(ctx, s.redisClient, cachePrefix, nextValues)
	if err != nil {
		return discord.NewError(replace, err)
	}

	return discord.NewResponse(webhookType, "").Ephemeral().AddEmbed(s.getQuoteEmbed(indexName, quote)).AddComponent(
		discord.Component{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, i18n[sendValue], sendKey),
				discord.NewButton(discord.SecondaryButton, i18n[nextValue], nextKey),
				discord.NewButton(discord.DangerButton, i18n[cancelValue], cancelAction),
			},
		})
}

func (s Service) quoteResponse(user, indexName string, quote model.Quote) (discord.InteractionResponse, bool, func(context.Context) discord.InteractionResponse) {
	return discord.NewReplace("Sending it..."), true, func(ctx context.Context) discord.InteractionResponse {
		return discord.NewResponse(discord.ChannelMessageWithSource, fmt.Sprintf("<@!%s> %s", user, i18n["title"])).AddEmbed(s.getQuoteEmbed(indexName, quote))
	}
}

func (s Service) getQuoteEmbed(indexName string, quote model.Quote) discord.Embed {
	switch indexName {
	case kaamelottName:
		return s.getKaamelottEmbeds(quote)

	case oss117Name:
		return s.getOss117Embeds(quote)

	case abitbolName:
		return s.getAbitbolEmbeds(quote)

	default:
		return discord.Embed{
			Title:       "Error",
			Description: fmt.Sprintf("render quote of index `%s`", indexName),
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

func (s Service) getAbitbolEmbeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		URL:         quote.URL,
		Thumbnail:   discord.NewImage(quote.Image),
	}
}
