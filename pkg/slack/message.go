package slack

import (
	"fmt"
	"net/url"

	"github.com/ViBiOh/kaamebott/pkg/model"
)

func (a App) getKaamelottBlock(output model.Quote) model.Block {
	titleLink := fmt.Sprintf("https://kaamelott-soundboard.2ec0b4.fr/#son/%s", url.PathEscape(output.ID))
	content := fmt.Sprintf("_%s_ %s", output.Character, output.Value)

	text := model.NewText(fmt.Sprintf("*<%s|%s>*\n\n%s", titleLink, output.Context, content))
	accessory := model.NewAccessory(fmt.Sprintf("%s/images/kaamelott.png", a.website), "kaamelott")

	return model.NewSection(text, accessory)
}

func (a App) getContentBlock(quote model.Quote) model.Block {
	switch quote.Collection {
	case "kaamelott":
		return a.getKaamelottBlock(quote)
	default:
		return model.EmptySection
	}
}
