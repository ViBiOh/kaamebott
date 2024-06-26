package main

import (
	"net/http"
)

func newPort(config configuration, services services) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/slack/", services.slack.NewServeMux())
	mux.Handle("/discord/", services.discord.NewServeMux())

	mux.Handle(config.renderer.PathPrefix+"/", services.renderer.NewServeMux(services.search.TemplateFunc))

	return mux
}
