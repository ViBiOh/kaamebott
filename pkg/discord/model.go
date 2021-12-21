package discord

type interactionType uint

const (
	pingInteraction               interactionType = 1
	applicationCommandInteraction interactionType = 2
	messageComponentInteraction   interactionType = 3
)

type callbackType uint

const (
	pongCallback                     callbackType = 1
	channelMessageWithSourceCallback callbackType = 4
	updateMessageCallback            callbackType = 7
)

type componentType uint

const (
	actionRowType componentType = 1
	buttonType    componentType = 2
)

type buttonStyle uint

const (
	primaryButton   buttonStyle = 1
	secondaryButton buttonStyle = 2
	dangerButton    buttonStyle = 4
)

const (
	ephemeralMessage int = 1 << 6
)

type interactionRequest struct {
	ID            string `json:"id"`
	GuildID       string `json:"guild_id"`
	Member        member `json:"member"`
	Token         string `json:"token"`
	ApplicationID string `json:"application_id"`
	Data          struct {
		Name     string          `json:"name"`
		CustomID string          `json:"custom_id"`
		Options  []commandOption `json:"options"`
	} `json:"data"`
	Type interactionType `json:"type"`
}

type member struct {
	User struct {
		ID       string `json:"id,omitempty"`
		Username string `json:"username,omitempty"`
	} `json:"user,omitempty"`
}

type interactionResponse struct {
	Data struct {
		Content         string         `json:"content,omitempty"`
		AllowedMentions allowedMention `json:"allowed_mentions"`
		Embeds          []embed        `json:"embeds"`
		Components      []component    `json:"components"`
		Flags           int            `json:"flags"`
	} `json:"data,omitempty"`
	Type callbackType `json:"type,omitempty"`
}

func newEphemeral(replace bool, content string) interactionResponse {
	callback := channelMessageWithSourceCallback
	if replace {
		callback = updateMessageCallback
	}

	instance := interactionResponse{Type: callback}
	instance.Data.Content = content
	instance.Data.Flags = ephemeralMessage
	instance.Data.Embeds = []embed{}
	instance.Data.Components = []component{}

	return instance
}

type allowedMention struct {
	Parse []string `json:"parse"`
}

type embed struct {
	Thumbnail   *embed  `json:"thumbnail,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	URL         string  `json:"url,omitempty"`
	Fields      []field `json:"fields,omitempty"`
	Color       int     `json:"color,omitempty"`
}

func (e embed) SetColor(color int) embed {
	e.Color = color
	return e
}

type field struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

func newField(name, value string) field {
	return field{
		Name:   name,
		Value:  value,
		Inline: true,
	}
}

type commandOption struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
	Type        int    `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type component struct {
	Label      string        `json:"label,omitempty"`
	CustomID   string        `json:"custom_id,omitempty"`
	Components []component   `json:"components,omitempty"`
	Type       componentType `json:"type,omitempty"`
	Style      buttonStyle   `json:"style,omitempty"`
}

func newButton(style buttonStyle, label, customID string) component {
	return component{
		Type:     buttonType,
		Style:    style,
		Label:    label,
		CustomID: customID,
	}
}

type command struct {
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Options     []commandOption `json:"options,omitempty"`
}
