# kaamebott

> Kaamebott adds a command in your Slack or Discord for finding an accurate quote from the Kaamelott world.

[![Build](https://github.com/ViBiOh/kaamebott/workflows/Build/badge.svg)](https://github.com/ViBiOh/kaamebott/actions)

## Thanks

Thanks to [@2ec0b4](https://github.com/2ec0b4/kaamelott-soundboard) for the awesome compilation job.

## Getting started

Golang binary is built with static link. You can download it directly from the [GitHub Release page](https://github.com/ViBiOh/kaamebott/releases) or build it by yourself by cloning this repo and running `make`.

You can configure app by passing CLI args or environment variables (cf. [Usage](#usage) section). CLI override environment variables.

You'll find a Kubernetes exemple in the [`infra/`](infra) folder, using my [`app chart`](https://github.com/ViBiOh/charts/tree/main/app)

## CI

Following variables are required for CI:

|            Name            |          Purpose          |
| :------------------------: | :-----------------------: |
| **SCRIPTS_NO_INTERACTIVE** | for running scripts in CI |

## Usage

The application can be configured by passing CLI args described below or their equivalent as environment variable. CLI values take precedence over environments variables.

Be careful when using the CLI values, if someone list the processes on the system, they will appear in plain-text. Pass secrets by environment variables: it's less easily visible.

```bash
Usage of kaamebott:
  --address               string        [server] Listen address ${KAAMEBOTT_ADDRESS}
  --cert                  string        [server] Certificate file ${KAAMEBOTT_CERT}
  --corsCredentials                     [cors] Access-Control-Allow-Credentials ${KAAMEBOTT_CORS_CREDENTIALS} (default false)
  --corsExpose            string        [cors] Access-Control-Expose-Headers ${KAAMEBOTT_CORS_EXPOSE}
  --corsHeaders           string        [cors] Access-Control-Allow-Headers ${KAAMEBOTT_CORS_HEADERS} (default "Content-Type")
  --corsMethods           string        [cors] Access-Control-Allow-Methods ${KAAMEBOTT_CORS_METHODS} (default "GET")
  --corsOrigin            string        [cors] Access-Control-Allow-Origin ${KAAMEBOTT_CORS_ORIGIN} (default "*")
  --csp                   string        [owasp] Content-Security-Policy ${KAAMEBOTT_CSP} (default "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com")
  --discordApplicationID  string        [discord] Application ID ${KAAMEBOTT_DISCORD_APPLICATION_ID}
  --discordBotToken       string        [discord] Bot Token ${KAAMEBOTT_DISCORD_BOT_TOKEN}
  --discordClientID       string        [discord] Client ID ${KAAMEBOTT_DISCORD_CLIENT_ID}
  --discordClientSecret   string        [discord] Client Secret ${KAAMEBOTT_DISCORD_CLIENT_SECRET}
  --discordPublicKey      string        [discord] Public Key ${KAAMEBOTT_DISCORD_PUBLIC_KEY}
  --frameOptions          string        [owasp] X-Frame-Options ${KAAMEBOTT_FRAME_OPTIONS} (default "deny")
  --graceDuration         duration      [http] Grace duration when signal received ${KAAMEBOTT_GRACE_DURATION} (default 30s)
  --hsts                                [owasp] Indicate Strict Transport Security ${KAAMEBOTT_HSTS} (default true)
  --idleTimeout           duration      [server] Idle Timeout ${KAAMEBOTT_IDLE_TIMEOUT} (default 2m0s)
  --key                   string        [server] Key file ${KAAMEBOTT_KEY}
  --loggerJson                          [logger] Log format as JSON ${KAAMEBOTT_LOGGER_JSON} (default false)
  --loggerLevel           string        [logger] Logger level ${KAAMEBOTT_LOGGER_LEVEL} (default "INFO")
  --loggerLevelKey        string        [logger] Key for level in JSON ${KAAMEBOTT_LOGGER_LEVEL_KEY} (default "level")
  --loggerMessageKey      string        [logger] Key for message in JSON ${KAAMEBOTT_LOGGER_MESSAGE_KEY} (default "msg")
  --loggerTimeKey         string        [logger] Key for timestamp in JSON ${KAAMEBOTT_LOGGER_TIME_KEY} (default "time")
  --minify                              Minify HTML ${KAAMEBOTT_MINIFY} (default true)
  --name                  string        [server] Name ${KAAMEBOTT_NAME} (default "http")
  --okStatus              int           [http] Healthy HTTP Status code ${KAAMEBOTT_OK_STATUS} (default 204)
  --pathPrefix            string        Root Path Prefix ${KAAMEBOTT_PATH_PREFIX}
  --port                  uint          [server] Listen port (0 to disable) ${KAAMEBOTT_PORT} (default 1080)
  --pprofAgent            string        [pprof] URL of the Datadog Trace Agent (e.g. http://datadog.observability:8126) ${KAAMEBOTT_PPROF_AGENT}
  --pprofPort             int           [pprof] Port of the HTTP server (0 to disable) ${KAAMEBOTT_PPROF_PORT} (default 0)
  --publicURL             string        Public URL ${KAAMEBOTT_PUBLIC_URL} (default "https://kaamebott.vibioh.fr")
  --readTimeout           duration      [server] Read Timeout ${KAAMEBOTT_READ_TIMEOUT} (default 5s)
  --redisAddress          string slice  [redis] Redis Address host:port (blank to disable) ${KAAMEBOTT_REDIS_ADDRESS}, as a string slice, environment variable separated by "," (default [127.0.0.1:6379])
  --redisDatabase         int           [redis] Redis Database ${KAAMEBOTT_REDIS_DATABASE} (default 0)
  --redisMinIdleConn      int           [redis] Redis Minimum Idle Connections ${KAAMEBOTT_REDIS_MIN_IDLE_CONN} (default 0)
  --redisPassword         string        [redis] Redis Password, if any ${KAAMEBOTT_REDIS_PASSWORD}
  --redisPoolSize         int           [redis] Redis Pool Size (default GOMAXPROCS*10) ${KAAMEBOTT_REDIS_POOL_SIZE} (default 0)
  --redisUsername         string        [redis] Redis Username, if any ${KAAMEBOTT_REDIS_USERNAME}
  --searchURL             string        [search] Meilisearch URL ${KAAMEBOTT_SEARCH_URL} (default "http://meilisearch:7700")
  --shutdownTimeout       duration      [server] Shutdown Timeout ${KAAMEBOTT_SHUTDOWN_TIMEOUT} (default 10s)
  --slackClientID         string        [slack] ClientID ${KAAMEBOTT_SLACK_CLIENT_ID}
  --slackClientSecret     string        [slack] ClientSecret ${KAAMEBOTT_SLACK_CLIENT_SECRET}
  --slackSigningSecret    string        [slack] Signing secret ${KAAMEBOTT_SLACK_SIGNING_SECRET}
  --telemetryRate         string        [telemetry] OpenTelemetry sample rate, 'always', 'never' or a float value ${KAAMEBOTT_TELEMETRY_RATE} (default "always")
  --telemetryURL          string        [telemetry] OpenTelemetry gRPC endpoint (e.g. otel-exporter:4317) ${KAAMEBOTT_TELEMETRY_URL}
  --telemetryUint64                     [telemetry] Change OpenTelemetry Trace ID format to an unsigned int 64 ${KAAMEBOTT_TELEMETRY_UINT64} (default true)
  --title                 string        Application title ${KAAMEBOTT_TITLE} (default "Kaamebott")
  --url                   string        [alcotest] URL to check ${KAAMEBOTT_URL}
  --userAgent             string        [alcotest] User-Agent for check ${KAAMEBOTT_USER_AGENT} (default "Alcotest")
  --writeTimeout          duration      [server] Write Timeout ${KAAMEBOTT_WRITE_TIMEOUT} (default 10s)
```
