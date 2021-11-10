SHELL = /usr/bin/env bash -o nounset -o pipefail -o errexit -c

ifneq ("$(wildcard .env)","")
	include .env
	export
endif

APP_NAME = kaamebott
PACKAGES ?= ./...

MAIN_SOURCE = cmd/kaamebott/kaamebott.go
MAIN_RUNNER = go run $(MAIN_SOURCE)
ifeq ($(DEBUG), true)
	MAIN_RUNNER = gdlv -d $(shell dirname $(MAIN_SOURCE)) debug --
endif

DISCORD_SOURCE = cmd/discord/discord.go
DISCORD_RUNNER = go run $(DISCORD_SOURCE)
ifeq ($(DEBUG), true)
	DISCORD_RUNNER = dlv debug $(DISCORD_SOURCE) --
endif

INDEXER_SOURCE = cmd/indexer/indexer.go
INDEXER_RUNNER = go run $(INDEXER_SOURCE)
ifeq ($(DEBUG), true)
	INDEXER_RUNNER = dlv debug $(INDEXER_SOURCE) --
endif

.DEFAULT_GOAL := app

## help: Display list of commands
.PHONY: help
help: Makefile
	@sed -n 's|^##||p' $< | column -t -s ':' | sort

## name: Output app name
.PHONY: name
name:
	@printf "$(APP_NAME)"

## version: Output last commit sha1
.PHONY: version
version:
	@printf "$(shell git rev-parse --short HEAD)"

## dev: Build app
.PHONY: dev
dev: format style test build

## app: Build whole app
.PHONY: app
app: init dev

## init: Bootstrap your application. e.g. fetch some data files, make some API calls, request user input etc...
.PHONY: init
init:
	@curl --disable --silent --show-error --location --max-time 30 "https://raw.githubusercontent.com/ViBiOh/scripts/main/bootstrap" | bash -s -- "-c" "git_hooks" "coverage"
	go install "github.com/kisielk/errcheck@latest"
	go install "golang.org/x/lint/golint@latest"
	go install "golang.org/x/tools/cmd/goimports@latest"
	go install "mvdan.cc/gofumpt@latest"
	go mod tidy -compat=1.17

## format: Format code. e.g Prettier (js), format (golang)
.PHONY: format
format:
	goimports -w $(shell find . -name "*.go")
	gofumpt -w $(shell find . -name "*.go")

## style: Check lint, code styling rules. e.g. pylint, phpcs, eslint, style (java) etc ...
.PHONY: style
style:
	golint $(PACKAGES)
	errcheck -ignoretests $(PACKAGES)
	go vet $(PACKAGES)

## test: Shortcut to launch all the test tasks (unit, functional and integration).
.PHONY: test
test:
	scripts/coverage
	go test $(PACKAGES) -bench . -benchmem -run Benchmark.*

## build: Build the application.
.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -installsuffix nocgo -o bin/$(APP_NAME) $(MAIN_SOURCE)
	CGO_ENABLED=0 go build -ldflags="-s -w" -installsuffix nocgo -o bin/discord $(DISCORD_SOURCE)

## run: Locally run the application, e.g. node index.js, python -m myapp, go run myapp etc ...
.PHONY: run
run:
	$(MAIN_RUNNER)

## run-discord: Locally run discord configuration
.PHONY: run-discord
run-discord:
	$(DISCORD_RUNNER)

## run-indexer: Locally run indexer
.PHONY: run-indexer
run-indexer:
	$(INDEXER_RUNNER) -input "$(INDEXER_FILE)"
