package main

import (
	"context"
	"embed"
	"flag"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/alcotest"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/db"
	"github.com/ViBiOh/httputils/v4/pkg/health"
	"github.com/ViBiOh/httputils/v4/pkg/httputils"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/prometheus"
	"github.com/ViBiOh/httputils/v4/pkg/recoverer"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/httputils/v4/pkg/tracer"
	"github.com/ViBiOh/kaamebott/pkg/quote"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

//go:embed templates static
var content embed.FS

const (
	slackPrefix   = "/slack"
	discordPrefix = "/discord"
)

func main() {
	fs := flag.NewFlagSet("kaamebott", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	appServerConfig := server.Flags(fs, "")
	promServerConfig := server.Flags(fs, "prometheus", flags.NewOverride("Port", uint(9090)), flags.NewOverride("IdleTimeout", 10*time.Second), flags.NewOverride("ShutdownTimeout", 5*time.Second))
	healthConfig := health.Flags(fs, "")

	alcotestConfig := alcotest.Flags(fs, "")
	loggerConfig := logger.Flags(fs, "logger")
	tracerConfig := tracer.Flags(fs, "tracer")
	prometheusConfig := prometheus.Flags(fs, "prometheus", flags.NewOverride("Gzip", false))
	owaspConfig := owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com"))
	corsConfig := cors.Flags(fs, "cors")
	rendererConfig := renderer.Flags(fs, "", flags.NewOverride("Title", "Kaamebott"), flags.NewOverride("PublicURL", "https://kaamebott.vibioh.fr"))

	redisConfig := redis.Flags(fs, "redis")

	searchConfig := search.Flags(fs, "search")
	slackConfig := slack.Flags(fs, "slack")
	discordConfig := discord.Flags(fs, "discord")

	dbConfig := db.Flags(fs, "db")

	logger.Fatal(fs.Parse(os.Args[1:]))

	alcotest.DoAndExit(alcotestConfig)
	logger.Global(logger.New(loggerConfig))
	defer logger.Close()

	ctx := context.Background()

	tracerApp, err := tracer.New(ctx, tracerConfig)
	logger.Fatal(err)
	defer tracerApp.Close(ctx)
	request.AddTracerToDefaultClient(tracerApp.GetProvider())

	quoteDB, err := db.New(ctx, dbConfig, tracerApp.GetTracer("database"))
	logger.Fatal(err)
	defer quoteDB.Close()

	appServer := server.New(appServerConfig)
	promServer := server.New(promServerConfig)
	prometheusApp := prometheus.New(prometheusConfig)
	healthApp := health.New(healthConfig, quoteDB.Ping)

	rendererApp, err := renderer.New(rendererConfig, content, search.FuncMap, tracerApp.GetTracer("renderer"))
	logger.Fatal(err)

	website := rendererApp.PublicURL("")

	redisApp, err := redis.New(redisConfig, tracerApp.GetProvider())
	logger.Fatal(err)
	defer redisApp.Close()

	searchApp := search.New(searchConfig, quoteDB, rendererApp)
	quoteApp := quote.New(website, searchApp, redisApp, tracerApp.GetTracer("quote"))

	discordApp, err := discord.New(discordConfig, website, quoteApp.DiscordHandler, tracerApp.GetTracer("discord"))
	logger.Fatal(err)

	slackHandler := http.StripPrefix(slackPrefix, slack.New(slackConfig, quoteApp.SlackCommand, quoteApp.SlackInteract, tracerApp.GetTracer("slack")).Handler())
	discordHandler := http.StripPrefix(discordPrefix, discordApp.Handler())
	kaamebottHandler := rendererApp.Handler(searchApp.TemplateFunc)

	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, slackPrefix) {
			slackHandler.ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, discordPrefix) {
			discordHandler.ServeHTTP(w, r)
		} else {
			kaamebottHandler.ServeHTTP(w, r)
		}
	})

	endCtx := healthApp.End(ctx)

	go promServer.Start(endCtx, "prometheus", prometheusApp.Handler())
	go appServer.Start(endCtx, "http", httputils.Handler(appHandler, healthApp, recoverer.Middleware, prometheusApp.Middleware, tracerApp.Middleware, owasp.New(owaspConfig).Middleware, cors.New(corsConfig).Middleware))

	healthApp.WaitForTermination(appServer.Done())
	server.GracefulWait(appServer.Done(), promServer.Done())
}
