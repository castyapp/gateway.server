package http

type Chat struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	Content   string   `json:"content"`
}
