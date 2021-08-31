package main

import (
	"flag"
	"os"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kaamebott/pkg/discord"
	"github.com/ViBiOh/kaamebott/pkg/search"
)

func main() {
	fs := flag.NewFlagSet("discord", flag.ExitOnError)

	loggerConfig := logger.Flags(fs, "logger")
	discordConfig := discord.Flags(fs, "")

	logger.Fatal(fs.Parse(os.Args[1:]))

	logger.Global(logger.New(loggerConfig))
	defer logger.Close()

	discordApp, err := discord.New(discordConfig, search.App{})
	logger.Fatal(err)

	logger.Fatal(discordApp.Start())
}
