package discord

import (
	"fmt"
	"net/url"

	"github.com/ViBiOh/kaamebott/pkg/model"
)

const (
	kaamelottName      = "kaamelott"
	kaamelottIndexName = "kaamelott"
)

func (a App) getKaamelottEmbeds(quote model.Quote) embed {
	return newEmbed(quote.Context, quote.Value, fmt.Sprintf("https://kaamelott-soundboard.2ec0b4.fr/#son/%s", url.PathEscape(quote.ID)), fmt.Sprintf("%s/images/kaamelott.jpg", a.website), newField("Personnage", quote.Character))
}
