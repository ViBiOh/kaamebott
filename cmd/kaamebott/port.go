package main

import (
	"net/http"

	"github.com/ViBiOh/httputils/v4/pkg/httputils"
)

func newPort(clients clients, services services) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/slack/", http.StripPrefix("/slack", services.slack.NewServeMux()))
	mux.Handle("/discord/", http.StripPrefix("/discord", services.discord.NewServeMux()))

	services.renderer.RegisterMux(mux, services.search.TemplateFunc)

	return httputils.Handler(mux, clients.health,
		clients.telemetry.Middleware("http"),
		services.owasp.Middleware,
		services.cors.Middleware,
	)
}
