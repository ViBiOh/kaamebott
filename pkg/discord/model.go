package discord

// webhook message
type webhook struct {
	Token   string `json:"token,omitempty"`
	ID      string `json:"id,omitempty"`
	GuildID string `json:"guild_id,omitempty"`
	Member  member `json:"member,omitempty"`
	Data    data   `json:"data,omitempty"`
	Type    int    `json:"type,omitempty"`
}

type member struct {
	User struct {
		Username string `json:"username,omitempty"`
	} `json:"user,omitempty"`
}

type field struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type embed struct {
	Thumbnail   *embed  `json:"thumbnail,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	URL         string  `json:"url,omitempty"`
	Fields      []field `json:"fields,omitempty"`
	Color       int     `json:"color,omitempty"`
}

type data struct {
	Name    string          `json:"name,omitempty"`
	Content string          `json:"content,omitempty"`
	Embeds  []embed         `json:"embeds,omitempty"`
	Options []commandOption `json:"options,omitempty"`
	TTS     bool            `json:"tts,omitempty"`
	Flags   int             `json:"flags,omitempty"`
}

type commandOption struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
	Type        int    `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type command struct {
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Options     []commandOption `json:"options,omitempty"`
}

func newEmbed(title, description, url, thumbnail string, fields ...field) embed {
	return embed{
		Title:       title,
		Description: description,
		URL:         url,
		Thumbnail: &embed{
			URL: thumbnail,
		},
		Fields: fields,
	}
}

func newField(name, value string) field {
	return field{
		Name:   name,
		Value:  value,
		Inline: true,
	}
}
