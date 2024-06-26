package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/kaamebott/pkg/quote"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

//go:embed templates static
var content embed.FS

type services struct {
	server   *server.Server
	renderer *renderer.Service

	search  search.Service
	discord discord.Service
	slack   slack.Service
}

func newServices(ctx context.Context, config configuration, clients clients) (services, error) {
	rendererService, err := renderer.New(ctx, config.renderer, content, search.FuncMap, clients.telemetry.MeterProvider(), clients.telemetry.TracerProvider())
	if err != nil {
		return services{}, fmt.Errorf("renderer: %w", err)
	}

	searchService := search.New(config.search, clients.db, rendererService)
	website := rendererService.PublicURL("")

	quoteService := quote.New(website, searchService, clients.redis, clients.telemetry.TracerProvider())

	discordService, err := discord.New(config.discord, website, quoteService.DiscordHandler, clients.telemetry.TracerProvider())
	if err != nil {
		return services{}, fmt.Errorf("discord: %w", err)
	}

	slackService := slack.New(config.slack, quoteService.SlackCommand, quoteService.SlackInteract, clients.telemetry.TracerProvider())

	return services{
		server:   server.New(config.server),
		renderer: rendererService,

		search:  searchService,
		discord: discordService,
		slack:   slackService,
	}, nil
}
