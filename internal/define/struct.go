package define

import "time"

type Response struct {
	Code   ErrorCode `json:"code"`
	Status string    `json:"status"`
	Date   time.Time `json:"date"`
	Data   []Item    `json:"data,omitempty"`
}

type Item struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Date        string `json:"date"`
	Author      string `json:"author,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
}
