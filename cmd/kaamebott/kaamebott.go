package main

import (
	"embed"
	"flag"
	"net/http"
	"os"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/alcotest"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/db"
	"github.com/ViBiOh/httputils/v4/pkg/flags"
	"github.com/ViBiOh/httputils/v4/pkg/health"
	"github.com/ViBiOh/httputils/v4/pkg/httputils"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/prometheus"
	"github.com/ViBiOh/httputils/v4/pkg/recoverer"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/kaamebott/pkg/discord"
	"github.com/ViBiOh/kaamebott/pkg/search"
	"github.com/ViBiOh/kaamebott/pkg/slack"
)

//go:embed templates static
var content embed.FS

const (
	slackPrefix   = "/slack"
	discordPrefix = "/discord"
)

func main() {
	fs := flag.NewFlagSet("kaamebott", flag.ExitOnError)

	appServerConfig := server.Flags(fs, "")
	promServerConfig := server.Flags(fs, "prometheus", flags.NewOverride("Port", 9090), flags.NewOverride("IdleTimeout", "10s"), flags.NewOverride("ShutdownTimeout", "5s"))
	healthConfig := health.Flags(fs, "")

	alcotestConfig := alcotest.Flags(fs, "")
	loggerConfig := logger.Flags(fs, "logger")
	prometheusConfig := prometheus.Flags(fs, "prometheus", flags.NewOverride("Gzip", false))
	owaspConfig := owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'nonce'; style-src 'self' 'nonce'; img-src 'self' platform.slack-edge.com"))
	corsConfig := cors.Flags(fs, "cors")
	rendererConfig := renderer.Flags(fs, "", flags.NewOverride("Title", "Kaamebott"), flags.NewOverride("PublicURL", "https://kaamebott.vibioh.fr"))

	searchConfig := search.Flags(fs, "search")
	slackConfig := slack.Flags(fs, "slack")
	discordConfig := discord.Flags(fs, "discord")

	dbConfig := db.Flags(fs, "db")

	logger.Fatal(fs.Parse(os.Args[1:]))

	alcotest.DoAndExit(alcotestConfig)
	logger.Global(logger.New(loggerConfig))
	defer logger.Close()

	quoteDB, err := db.New(dbConfig)
	logger.Fatal(err)
	defer quoteDB.Close()

	appServer := server.New(appServerConfig)
	promServer := server.New(promServerConfig)
	prometheusApp := prometheus.New(prometheusConfig)
	healthApp := health.New(healthConfig, quoteDB.Ping)

	rendererApp, err := renderer.New(rendererConfig, content, search.FuncMap)
	logger.Fatal(err)

	searchApp := search.New(searchConfig, quoteDB, rendererApp)

	discordApp, err := discord.New(discordConfig, searchApp)
	logger.Fatal(err)

	slackHandler := http.StripPrefix(slackPrefix, slack.New(slackConfig, searchApp).Handler())
	discordHandler := http.StripPrefix(discordPrefix, discordApp.Handler())
	rendererHandler := rendererApp.Handler(searchApp.TemplateFunc)

	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, slackPrefix) {
			slackHandler.ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, discordPrefix) {
			discordHandler.ServeHTTP(w, r)
		} else {
			rendererHandler.ServeHTTP(w, r)
		}
	})

	go promServer.Start("prometheus", healthApp.End(), prometheusApp.Handler())
	go appServer.Start("http", healthApp.End(), httputils.Handler(appHandler, healthApp, recoverer.Middleware, prometheusApp.Middleware, owasp.New(owaspConfig).Middleware, cors.New(corsConfig).Middleware))

	healthApp.WaitForTermination(appServer.Done())
	server.GracefulWait(appServer.Done(), promServer.Done())
}
