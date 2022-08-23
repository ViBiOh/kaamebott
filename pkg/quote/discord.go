package quote

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
	"github.com/ViBiOh/httputils/v4/pkg/tracer"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
	"github.com/ViBiOh/kaamebott/pkg/version"
)

const (
	queryParam       = "recherche"
	contentSeparator = "@"

	kaamelottName          = "kaamelott"
	kaamelottGifName       = "kaamelottgif"
	kaamelottGifCollection = "kaamelott_gif"
	oss117Name             = "oss117"
	officeName             = "office"
	filmsName              = "citation"
	filmsCollection        = "films"
)

var indexes = map[string]string{
	kaamelottName:    kaamelottName,
	kaamelottGifName: kaamelottGifCollection,
	oss117Name:       oss117Name,
	officeName:       officeName,
	filmsName:        filmsCollection,
}

// DiscordHandler handle discord request
func (a App) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, func(context.Context) discord.InteractionResponse) {
	ctx, end := tracer.StartSpan(ctx, a.tracer, "DiscordHandler")
	defer end()

	index, err := a.checkRequest(webhook)
	if err != nil {
		return discord.NewEphemeral(false, err.Error()), nil
	}

	action, query, next, err := a.getQuery(ctx, webhook)
	if err != nil {
		return discord.NewEphemeral(false, err.Error()), nil
	}

	switch action {
	case nextValue:
		return a.handleSearch(ctx, index, query, next), nil

	case sendValue:
		quote, err := a.searchApp.GetByID(ctx, index, query)
		if err != nil {
			return discord.NewError(true, err), nil
		}

		return a.quoteResponse(webhook.Member.User.ID, quote), nil

	case cancelValue:
		return discord.NewEphemeral(true, "Ok, not now."), nil

	default:
		return a.handleSearch(ctx, index, query, ""), nil
	}
}

func (a App) checkRequest(webhook discord.InteractionRequest) (string, error) {
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

func (a App) getQuery(ctx context.Context, webhook discord.InteractionRequest) (string, string, string, error) {
	switch webhook.Type {
	case discord.MessageComponentInteraction:
		if webhook.Data.CustomID == cancelValue {
			return cancelValue, "", "", nil
		}

		content, err := a.redisApp.Load(ctx, version.Redis(webhook.Data.CustomID))
		if err != nil {
			return "", "", "", fmt.Errorf("load custom id: %w", err)
		}

		parts := strings.SplitN(content, contentSeparator, 3)
		switch parts[0] {
		case sendValue:
			if len(parts) != 2 {
				return "", "", "", errors.New("part send value")
			}

			return sendValue, parts[1], "", nil
		case nextValue:
			if len(parts) != 3 {
				return "", "", "", errors.New("part cancel value")
			}

			return sendValue, parts[1], parts[2], nil
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

func (a App) handleSearch(ctx context.Context, index, query, last string) discord.InteractionResponse {
	quote, err := a.searchApp.Search(ctx, index, query, last)
	if err != nil && !errors.Is(err, search.ErrNotFound) {
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("Oh, it's broken 😱. Reason: %s", err))
	}

	if len(quote.ID) == 0 {
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("We found nothing for `%s`", query))
	}

	return a.interactiveResponse(ctx, quote, len(last) != 0, query)
}

func (a App) interactiveResponse(ctx context.Context, quote model.Quote, replace bool, recherche string) discord.InteractionResponse {
	ctx, end := tracer.StartSpan(ctx, a.tracer, "interactiveResponse")
	defer end()

	webhookType := discord.ChannelMessageWithSource
	if replace {
		webhookType = discord.UpdateMessageCallback
	}

	sendContent := strings.Join([]string{sendValue, quote.ID}, contentSeparator)
	sendSha := sha.New(sendContent)
	if err := a.redisApp.Store(ctx, version.Redis(sendSha), sendContent, time.Hour); err != nil {
		return discord.NewError(replace, err)
	}

	nextContent := strings.Join([]string{nextValue, recherche, quote.ID}, contentSeparator)
	nextSha := sha.New(nextContent)
	if err := a.redisApp.Store(ctx, version.Redis(nextSha), nextContent, time.Hour); err != nil {
		return discord.NewError(replace, err)
	}

	return discord.NewResponse(webhookType, "").Ephemeral().AddEmbed(a.getQuoteEmbed(quote)).AddComponent(
		discord.Component{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, i18n[quote.Language][sendValue], sendSha),
				discord.NewButton(discord.SecondaryButton, i18n[quote.Language][nextValue], nextSha),
				discord.NewButton(discord.DangerButton, i18n[quote.Language][cancelValue], cancelValue),
			},
		})
}

func (a App) quoteResponse(user string, quote model.Quote) discord.InteractionResponse {
	return discord.NewResponse(discord.ChannelMessageWithSource, fmt.Sprintf("<@!%s> %s", user, i18n[quote.Language]["title"])).AddEmbed(a.getQuoteEmbed(quote))
}

func (a App) getQuoteEmbed(quote model.Quote) discord.Embed {
	switch quote.Collection {
	case kaamelottName:
		return a.getKaamelottEmbeds(quote)
	case kaamelottGifCollection:
		return a.getKaamelottGifEmbeds(quote)
	case oss117Name:
		return a.getOss117Embeds(quote)
	case officeName:
		return a.getOfficeEmbeds(quote)
	case filmsCollection:
		return a.getFilmsEmbeds(quote)
	default:
		return discord.Embed{
			Title:       "Error",
			Description: fmt.Sprintf("render quote of collection `%s`", quote.Collection),
		}
	}
}

func (a App) getKaamelottEmbeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		URL:         quote.URL,
		Thumbnail:   discord.NewImage(fmt.Sprintf("%s/images/kaamelott.png", a.website)),
		Fields: []discord.Field{
			discord.NewField("Personnage", quote.Character),
		},
	}
}

func (a App) getKaamelottGifEmbeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Image: discord.NewImage(quote.URL),
		Fields: []discord.Field{
			discord.NewField("Personnage", quote.Character),
		},
	}
}

func (a App) getOss117Embeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		Thumbnail:   discord.NewImage(fmt.Sprintf("%s/images/oss117.png", a.website)),
		Fields: []discord.Field{
			discord.NewField("Personnage", quote.Character),
		},
	}
}

func (a App) getOfficeEmbeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		Thumbnail:   discord.NewImage(fmt.Sprintf("%s/images/office.jpg", a.website)),
	}
}

func (a App) getFilmsEmbeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		Fields: []discord.Field{
			discord.NewField("Personnage(s)", quote.Character),
		},
	}
}
