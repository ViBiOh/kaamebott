package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/kaamebott/pkg/quote"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

//go:embed templates static
var content embed.FS

type services struct {
	server   *server.Server
	owasp    owasp.Service
	cors     cors.Service
	renderer *renderer.Service

	search  search.Service
	discord discord.Service
	slack   slack.Service
}

func newServices(ctx context.Context, config configuration, clients clients) (services, error) {
	var output services
	var err error

	output.server = server.New(config.server)
	output.owasp = owasp.New(config.owasp)
	output.cors = cors.New(config.cors)

	output.renderer, err = renderer.New(ctx, config.renderer, content, search.FuncMap, clients.telemetry.MeterProvider(), clients.telemetry.TracerProvider())
	if err != nil {
		return output, fmt.Errorf("renderer: %w", err)
	}

	output.search = search.New(config.search, output.renderer)

	website := output.renderer.PublicURL("")
	quoteService := quote.New(website, output.search, clients.redis, clients.telemetry.TracerProvider())

	output.discord, err = discord.New(config.discord, website, quoteService.DiscordHandler, clients.telemetry.TracerProvider())
	if err != nil {
		return output, fmt.Errorf("discord: %w", err)
	}

	output.slack = slack.New(config.slack, quoteService.SlackCommand, quoteService.SlackInteract, clients.telemetry.TracerProvider())

	return output, nil
}
