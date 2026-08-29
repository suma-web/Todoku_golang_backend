package search

type Result struct {
	Type       string `json:"type"`
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Excerpt    string `json:"excerpt"`
	Category   string `json:"category,omitempty"`
	Department string `json:"department,omitempty"`
}

type Response struct {
	Query   string   `json:"query"`
	Results []Result `json:"results"`
}
