package discord

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
)

var (
	commands = map[string]command{
		kaamelottName: {
			Name:        kaamelottName,
			Description: "Une citation de la cour du roi Arthur",
			Options: []commandOption{
				{
					Name:        queryParam,
					Description: "Un mot clé pour affiner la recherche",
					Type:        3, // https://discord.com/developers/docs/interactions/slash-commands#applicationcommandoptiontype
					Required:    true,
				},
			},
		},
	}

	indexes = map[string]string{
		kaamelottName: kaamelottIndexName,
	}
)

// Start discord configuration
func (a App) Start() error {
	if len(a.applicationID) == 0 {
		return nil
	}

	ctx := context.Background()

	data := url.Values{}
	data.Add("grant_type", "client_credentials")
	data.Add("scope", "applications.commands.update")

	resp, err := discordRequest.Method(http.MethodPost).Path("/oauth2/token").BasicAuth(a.clientID, a.clientSecret).Form(ctx, data)
	if err != nil {
		return fmt.Errorf("unable to get token: %s", err)
	}

	content := make(map[string]any)
	if err := httpjson.Read(resp, &content); err != nil {
		return fmt.Errorf("unable to read oauth token: %s", err)
	}

	bearer := content["access_token"].(string)
	url := fmt.Sprintf("/applications/%s/commands", a.applicationID)

	for name, command := range commands {
		logger.WithField("command", name).Info("Configuring with URL `%s`", url)

		_, err := discordRequest.Method(http.MethodPost).Path(url).Header("Authorization", fmt.Sprintf("Bearer %s", bearer)).JSON(ctx, command)
		if err != nil {
			return fmt.Errorf("unable to configure `%s` command: %s", name, err)
		}

		logger.Info("Command `%s` configured!", name)
	}

	return nil
}
