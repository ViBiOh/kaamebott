package quote

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

const (
	queryParam       = "recherche"
	contentSeparator = "@"

	kaamelottName          = "kaamelott"
	kaamelottGifName       = "kaamelottgif"
	kaamelottGifCollection = "kaamelott_gif"
	oss117Name             = "oss117"
	officeName             = "office"
)

var indexes = map[string]string{
	kaamelottName:    kaamelottName,
	kaamelottGifName: kaamelottGifCollection,
	oss117Name:       oss117Name,
	officeName:       officeName,
}

// DiscordHandler handle discord request
func (a App) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, func(context.Context) discord.InteractionResponse) {
	index, err := a.checkRequest(webhook)
	if err != nil {
		return discord.NewEphemeral(false, err.Error()), nil
	}

	queryValue := a.getQuery(webhook)
	switch strings.Count(queryValue, contentSeparator) {
	case 0:
		return a.handleSearch(ctx, index, queryValue, ""), nil
	case 1:
		var last string
		lastIndex := strings.LastIndexAny(queryValue, contentSeparator)
		last = queryValue[lastIndex+1:]
		queryValue = queryValue[:lastIndex]
		return a.handleSearch(ctx, index, queryValue, last), nil
	case 2:
		quote, err := a.searchApp.GetByID(ctx, index, strings.Trim(queryValue, contentSeparator))
		if err != nil {
			return discord.NewEphemeral(true, err.Error()), nil
		}

		return a.quoteResponse(webhook.Member.User.ID, quote), nil
	case 3:
		return discord.NewEphemeral(true, "Ok, not now."), nil
	default:
		return discord.NewEphemeral(true, "Unknown behavior."), nil
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

func (a App) getQuery(webhook discord.InteractionRequest) string {
	switch webhook.Type {
	case discord.MessageComponentInteraction:
		return webhook.Data.CustomID
	case discord.ApplicationCommandInteraction:
		for _, option := range webhook.Data.Options {
			if strings.EqualFold(option.Name, queryParam) {
				return option.Value
			}
		}
	}

	return ""
}

func (a App) handleSearch(ctx context.Context, index, query, last string) discord.InteractionResponse {
	quote, err := a.searchApp.Search(ctx, index, query, last)
	if err != nil && !errors.Is(err, search.ErrNotFound) {
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("Oh, it's broken 😱. Reason: %s", err))
	}

	if len(quote.ID) == 0 {
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("We found nothing for `%s`", query))
	}

	return a.interactiveResponse(quote, len(last) != 0, query)
}

func (a App) interactiveResponse(quote model.Quote, replace bool, recherche string) discord.InteractionResponse {
	webhookType := discord.ChannelMessageWithSource
	if replace {
		webhookType = discord.UpdateMessageCallback
	}

	instance := discord.InteractionResponse{Type: webhookType}
	instance.Data.Flags = discord.EphemeralMessage
	instance.Data.Embeds = []discord.Embed{a.getQuoteEmbed(quote)}
	instance.Data.Components = []discord.Component{
		{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, i18n[quote.Language][sendValue], fmt.Sprintf("%s%s%s", contentSeparator, quote.ID, contentSeparator)),
				discord.NewButton(discord.SecondaryButton, i18n[quote.Language][nextValue], fmt.Sprintf("%s%s%s", recherche, contentSeparator, quote.ID)),
				discord.NewButton(discord.DangerButton, i18n[quote.Language][cancelValue], fmt.Sprintf("%s%s%s", contentSeparator, contentSeparator, contentSeparator)),
			},
		},
	}

	return instance
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
	default:
		return discord.Embed{
			Title:       "Error",
			Description: fmt.Sprintf("unable to render quote of collection `%s`", quote.Collection),
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
