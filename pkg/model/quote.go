package model

type Quote struct {
	ID        string `json:"id"`
	Value     string `json:"value"`
	Character string `json:"character"`
	Context   string `json:"context"`
	URL       string `json:"url"`
	Image     string `json:"image"`
}
