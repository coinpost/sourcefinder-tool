package grok

import "time"

// Response represents a Grok response
type Response struct {
	Success  bool     `json:"success"`
	Response GrokMsg  `json:"response,omitempty"`
	Metadata Metadata `json:"metadata,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// GrokMsg represents a Grok message
type GrokMsg struct {
	Text      string    `json:"text"`
	HTML      string    `json:"html,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	TurnIndex int       `json:"turn_index,omitempty"`
}

// Metadata contains additional information about the response
type Metadata struct {
	DocumentPath   string        `json:"document_path,omitempty"`
	Duration       time.Duration `json:"duration_ms"`
	ScreenshotPath string        `json:"screenshot_path,omitempty"`
	ToolCalls      int           `json:"tool_calls,omitempty"`
}
