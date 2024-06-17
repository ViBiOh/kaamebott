package main

import (
	"net/http"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

func newPort(rendererService *renderer.Service, searchService search.Service, slackService slack.Service, discordService discord.Service) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/slack", http.StripPrefix("/slack", slackService.Handler()))
	mux.Handle("/discord", http.StripPrefix("/discord", discordService.Handler()))

	rendererService.Register(mux, searchService.TemplateFunc)

	return mux
}
