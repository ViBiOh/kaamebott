package main

import (
	"context"
	"embed"
	"flag"
	"os"

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
	"github.com/ViBiOh/httputils/v4/pkg/pprof"
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

func main() {
	fs := flag.NewFlagSet("kaamebott", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	appServerConfig := server.Flags(fs, "")
	healthConfig := health.Flags(fs, "")

	alcotestConfig := alcotest.Flags(fs, "")
	loggerConfig := logger.Flags(fs, "logger")
	telemetryConfig := telemetry.Flags(fs, "telemetry")
	pprofConfig := pprof.Flags(fs, "pprof")
	owaspConfig := owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com"))
	corsConfig := cors.Flags(fs, "cors")
	rendererConfig := renderer.Flags(fs, "", flags.NewOverride("Title", "Kaamebott"), flags.NewOverride("PublicURL", "https://kaamebott.vibioh.fr"))

	redisConfig := redis.Flags(fs, "redis")

	searchConfig := search.Flags(fs, "search")
	slackConfig := slack.Flags(fs, "slack")
	discordConfig := discord.Flags(fs, "discord")

	dbConfig := db.Flags(fs, "db")

	_ = fs.Parse(os.Args[1:])

	alcotest.DoAndExit(alcotestConfig)

	ctx := context.Background()

	logger.Init(ctx, loggerConfig)

	telemetryService, err := telemetry.New(ctx, telemetryConfig)
	logger.FatalfOnErr(ctx, err, "create telemetry")

	defer telemetryService.Close(ctx)

	logger.AddOpenTelemetryToDefaultLogger(telemetryService)
	request.AddOpenTelemetryToDefaultClient(telemetryService.MeterProvider(), telemetryService.TracerProvider())

	service, version, envName := telemetryService.GetServiceVersionAndEnv()
	pprofApp := pprof.New(pprofConfig, service, version, envName)

	appServer := server.New(appServerConfig)

	quoteDB, err := db.New(ctx, dbConfig, telemetryService.TracerProvider())
	logger.FatalfOnErr(ctx, err, "create database")

	defer quoteDB.Close()

	healthService := health.New(ctx, healthConfig, quoteDB.Ping)

	go pprofApp.Start(healthService.DoneCtx())

	rendererService, err := renderer.New(ctx, rendererConfig, content, search.FuncMap, telemetryService.MeterProvider(), telemetryService.TracerProvider())
	logger.FatalfOnErr(ctx, err, "create renderer")

	website := rendererService.PublicURL("")

	redisClient, err := redis.New(ctx, redisConfig, telemetryService.MeterProvider(), telemetryService.TracerProvider())
	logger.FatalfOnErr(ctx, err, "create redis")

	defer redisClient.Close(ctx)

	searchService := search.New(searchConfig, quoteDB, rendererService)
	quoteService := quote.New(website, searchService, redisClient, telemetryService.TracerProvider())

	discordService, err := discord.New(discordConfig, website, quoteService.DiscordHandler, telemetryService.TracerProvider())
	logger.FatalfOnErr(ctx, err, "create discord")

	slackService := slack.New(slackConfig, quoteService.SlackCommand, quoteService.SlackInteract, telemetryService.TracerProvider())

	port := newPort(rendererService, searchService, slackService, discordService)

	go appServer.Start(healthService.EndCtx(), httputils.Handler(port, healthService, telemetryService.Middleware("http"), owasp.New(owaspConfig).Middleware, cors.New(corsConfig).Middleware))

	healthService.WaitForTermination(appServer.Done())

	server.GracefulWait(appServer.Done())
}
