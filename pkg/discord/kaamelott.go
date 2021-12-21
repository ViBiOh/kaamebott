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
	return embed{
		Title:       quote.Context,
		Description: quote.Value,
		URL:         fmt.Sprintf("https://kaamelott-soundboard.2ec0b4.fr/#son/%s", url.PathEscape(quote.ID)),
		Thumbnail: &embed{
			URL: fmt.Sprintf("%s/images/kaamelott.png", a.website),
		},
		Fields: []field{
			newField("Personnage", quote.Character),
		},
	}
}
