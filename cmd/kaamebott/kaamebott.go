package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

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
	"github.com/ViBiOh/httputils/v4/pkg/recoverer"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/httputils/v4/pkg/telemetry"
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
	healthConfig := health.Flags(fs, "")

	alcotestConfig := alcotest.Flags(fs, "")
	loggerConfig := logger.Flags(fs, "logger")
	telemetryConfig := telemetry.Flags(fs, "telemetry")
	owaspConfig := owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com"))
	corsConfig := cors.Flags(fs, "cors")
	rendererConfig := renderer.Flags(fs, "", flags.NewOverride("Title", "Kaamebott"), flags.NewOverride("PublicURL", "https://kaamebott.vibioh.fr"))

	redisConfig := redis.Flags(fs, "redis")

	searchConfig := search.Flags(fs, "search")
	slackConfig := slack.Flags(fs, "slack")
	discordConfig := discord.Flags(fs, "discord")

	dbConfig := db.Flags(fs, "db")

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	alcotest.DoAndExit(alcotestConfig)

	logger.Init(loggerConfig)

	ctx := context.Background()

	telemetryService, err := telemetry.New(ctx, telemetryConfig)
	if err != nil {
		slog.Error("create telemetry", "err", err)
		os.Exit(1)
	}

	defer telemetryService.Close(ctx)
	request.AddOpenTelemetryToDefaultClient(telemetryService.MeterProvider(), telemetryService.TracerProvider())

	quoteDB, err := db.New(ctx, dbConfig, telemetryService.TracerProvider())
	if err != nil {
		slog.Error("create database", "err", err)
		os.Exit(1)
	}

	defer quoteDB.Close()

	appServer := server.New(appServerConfig)
	healthService := health.New(ctx, healthConfig, quoteDB.Ping)

	rendererService, err := renderer.New(rendererConfig, content, search.FuncMap, telemetryService.MeterProvider(), telemetryService.TracerProvider())
	if err != nil {
		slog.Error("create renderer", "err", err)
		os.Exit(1)
	}

	website := rendererService.PublicURL("")

	redisClient, err := redis.New(redisConfig, telemetryService.MeterProvider(), telemetryService.TracerProvider())
	if err != nil {
		slog.Error("create redis", "err", err)
		os.Exit(1)
	}

	defer redisClient.Close()

	searchService := search.New(searchConfig, quoteDB, rendererService)
	quoteService := quote.New(website, searchService, redisClient, telemetryService.TracerProvider())

	discordService, err := discord.New(discordConfig, website, quoteService.DiscordHandler, telemetryService.TracerProvider())
	if err != nil {
		slog.Error("create discord", "err", err)
		os.Exit(1)
	}

	slackHandler := http.StripPrefix(slackPrefix, slack.New(slackConfig, quoteService.SlackCommand, quoteService.SlackInteract, telemetryService.TracerProvider()).Handler())
	discordHandler := http.StripPrefix(discordPrefix, discordService.Handler())
	kaamebottHandler := rendererService.Handler(searchService.TemplateFunc)

	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, slackPrefix) {
			slackHandler.ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, discordPrefix) {
			discordHandler.ServeHTTP(w, r)
		} else {
			kaamebottHandler.ServeHTTP(w, r)
		}
	})

	go appServer.Start(healthService.EndCtx(), "http", httputils.Handler(appHandler, healthService, recoverer.Middleware, telemetryService.Middleware("http"), owasp.New(owaspConfig).Middleware, cors.New(corsConfig).Middleware))

	healthService.WaitForTermination(appServer.Done())

	appServer.Stop(ctx)
}
