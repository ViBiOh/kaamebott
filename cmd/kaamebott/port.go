package main

import (
	"net/http"
)

func newPort(config configuration, services services) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/slack/", http.StripPrefix("/slack", services.slack.NewServeMux()))
	mux.Handle("/discord/", http.StripPrefix("/discord", services.discord.NewServeMux()))

	mux.Handle(config.renderer.PathPrefix+"/", http.StripPrefix(
		config.renderer.PathPrefix,
		services.renderer.NewServeMux(services.search.TemplateFunc),
	))

	return mux
}
