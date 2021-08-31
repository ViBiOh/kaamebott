package slack

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kaamebott/pkg/model"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

const (
	cancelValue = "cancel"
	nextValue   = "next"
	sendValue   = "send"
)

var (
	cancelButton = model.NewButtonElement("Annuler", cancelValue, "", "danger")
)

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

func (a App) getQuoteBlock(ctx context.Context, index, text string, last string) model.Response {
	quote, err := a.getQuote(ctx, index, text, last)
	if err != nil {
		return model.NewEphemeralMessage(fmt.Sprintf("Ah, c'est cassé 😱. La raison : %s", err))
	}

	return a.getQuoteResponse(quote, text, "")
}

func (a App) getQuoteResponse(quote model.Quote, query, user string) model.Response {
	content := a.getContentBlock(quote)
	if content == model.EmptySection {
		return model.NewEphemeralMessage(fmt.Sprintf("On n'a rien trouvé pour `%s`", query))
	}

	if len(user) == 0 {
		return model.Response{
			ResponseType:    "ephemeral",
			ReplaceOriginal: true,
			Blocks: []model.Block{
				content,
				model.NewActions(quote.Collection, cancelButton, model.NewButtonElement("Une autre ?", nextValue, fmt.Sprintf("%s_%s", query, quote.ID), ""),
					model.NewButtonElement("Envoyer", sendValue, quote.ID, "primary")),
			},
		}
	}

	return model.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []model.Block{
			model.NewSection(model.NewText(fmt.Sprintf("<@%s> vous partage une petite _quote_  ", user)), nil),
			content,
		},
	}
}

func (a App) handleQuote(w http.ResponseWriter, r *http.Request, index string) {
	query := r.FormValue("text")

	logger.WithField("command", r.FormValue("command")).WithField("query", query).WithField("user", r.FormValue("user_name")).Info("Slack call")

	httpjson.Write(w, http.StatusOK, a.getQuoteBlock(r.Context(), index, query, ""))
}

func (a App) handleQuoteInteract(r *http.Request, user string, actions []model.InteractiveAction) model.Response {
	action := actions[0]
	if action.ActionID == cancelValue {
		return model.NewEphemeralMessage("Ok, pas maintenant.")
	}

	ctx := context.Background()

	if action.ActionID == sendValue {
		quote, err := a.searchApp.GetByID(ctx, action.BlockID, action.Value)
		if err != nil {
			return model.NewEphemeralMessage(fmt.Sprintf("Impossible de retrouver la citation demandée: %s", err))
		}

		return a.getQuoteResponse(quote, "", user)
	}

	if action.ActionID == nextValue {
		parts := strings.Split(action.Value, "_")
		if len(parts) < 2 {
			return model.NewEphemeralMessage(fmt.Sprintf("La valeur du bouton semble incomplète: %s", action.Value))
		}

		return a.getQuoteBlock(ctx, action.BlockID, parts[0], parts[1])
	}

	return model.NewEphemeralMessage("On ne comprend pas l'action à effectuer")
}
