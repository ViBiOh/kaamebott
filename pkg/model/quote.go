package model

type Quote struct {
	ID         string `json:"id"`
	Value      string `json:"value"`
	Character  string `json:"character"`
	Context    string `json:"context"`
	Collection string `json:"collection"`
	Language   string `json:"language"`
	URL        string `json:"url"`
	Image      string `json:"image"`
}
