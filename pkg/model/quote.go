package model

// Quote is someone who say something
type Quote struct {
	ID         string `json:"id"`
	Value      string `json:"value"`
	Character  string `json:"character"`
	Context    string `json:"context"`
	Collection string `json:"collection"`
}
