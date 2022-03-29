package quote

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/kaamebott/pkg/discord"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

const (
	queryParam       = "recherche"
	contentSeparator = "@"

	kaamelottName      = "kaamelott"
	kaamelottIndexName = "kaamelott"
)

var (
	// Commands configuration
	Commands = map[string]discord.Command{
		kaamelottName: {
			Name:        kaamelottName,
			Description: "Une citation de la cour du roi Arthur",
			Options: []discord.CommandOption{
				{
					Name:        queryParam,
					Description: "Un mot clé pour affiner la recherche",
					Type:        3, // https://discord.com/developers/docs/interactions/slash-commands#applicationcommandoptiontype
					Required:    true,
				},
			},
		},
	}

	indexes = map[string]string{
		kaamelottName: kaamelottIndexName,
	}
)

// DiscordHandler handle discord request
func (a App) DiscordHandler(w http.ResponseWriter, r *http.Request, webhook discord.InteractionRequest) {
	index, err := a.checkRequest(webhook)
	if err != nil {
		respond(w, discord.NewEphemeral(false, err.Error()))
		return
	}

	queryValue := a.getQuery(webhook)
	switch strings.Count(queryValue, contentSeparator) {
	case 0:
		respond(w, a.handleSearch(r.Context(), index, queryValue, ""))

	case 1:
		var last string
		lastIndex := strings.LastIndexAny(queryValue, contentSeparator)
		last = queryValue[lastIndex+1:]
		queryValue = queryValue[:lastIndex]
		respond(w, a.handleSearch(r.Context(), index, queryValue, last))

	case 2:
		quote, err := a.searchApp.GetByID(r.Context(), index, strings.Trim(queryValue, contentSeparator))
		if err != nil {
			respond(w, discord.NewEphemeral(true, err.Error()))
			return
		}

		respond(w, a.quoteResponse(webhook.Member.User.ID, quote))

	case 3:
		respond(w, discord.NewEphemeral(true, "Ok, pas maintenant."))
	}
}

func (a App) checkRequest(webhook discord.InteractionRequest) (string, error) {
	var index string
	switch webhook.Type {
	case discord.MessageComponentInteraction:
		index = webhook.Message.Interaction.Name
	case discord.ApplicationCommandInteraction:
		index = webhook.Data.Name
	}

	index, ok := indexes[index]
	if !ok {
		return "", fmt.Errorf("unknown command `%s`", index)
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
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("Ah, c'est cassé 😱. La raison : %s", err))
	}

	if len(quote.ID) == 0 {
		return discord.NewEphemeral(len(last) != 0, fmt.Sprintf("On n'a rien trouvé pour `%s`", query))
	}

	return a.interactiveResponse(quote, len(last) != 0, query)
}

func (a App) interactiveResponse(quote model.Quote, replace bool, recherche string) discord.InteractionResponse {
	webhookType := discord.ChannelMessageWithSourceCallback
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
				discord.NewButton(discord.PrimaryButton, "Envoyer", fmt.Sprintf("%s%s%s", contentSeparator, quote.ID, contentSeparator)),
				discord.NewButton(discord.SecondaryButton, "Une autre ?", fmt.Sprintf("%s%s%s", recherche, contentSeparator, quote.ID)),
				discord.NewButton(discord.DangerButton, "Annuler", fmt.Sprintf("%s%s%s", contentSeparator, contentSeparator, contentSeparator)),
			},
		},
	}

	return instance
}

func (a App) quoteResponse(user string, quote model.Quote) discord.InteractionResponse {
	instance := discord.InteractionResponse{Type: discord.ChannelMessageWithSourceCallback}
	instance.Data.Content = fmt.Sprintf("<@!%s> vous partage une petite quote", user)
	instance.Data.AllowedMentions = discord.AllowedMention{
		Parse: []string{},
	}
	instance.Data.Embeds = []discord.Embed{a.getQuoteEmbed(quote)}

	return instance
}

func (a App) getQuoteEmbed(quote model.Quote) discord.Embed {
	switch quote.Collection {
	case kaamelottIndexName:
		return a.getKaamelottEmbeds(quote)
	default:
		return discord.Embed{
			Title:       "Error",
			Description: fmt.Sprintf("unable to render quote of collection `%s`", quote.Collection),
		}
	}
}

func respond(w http.ResponseWriter, response discord.InteractionResponse) {
	httpjson.Write(w, http.StatusOK, response)
}

func (a App) getKaamelottEmbeds(quote model.Quote) discord.Embed {
	return discord.Embed{
		Title:       quote.Context,
		Description: quote.Value,
		URL:         fmt.Sprintf("https://kaamelott-soundboard.2ec0b4.fr/#son/%s", url.PathEscape(quote.ID)),
		Thumbnail: &discord.Embed{
			URL: fmt.Sprintf("%s/images/kaamelott.png", a.website),
		},
		Fields: []discord.Field{
			discord.NewField("Personnage", quote.Character),
		},
	}
}
