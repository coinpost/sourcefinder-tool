package chatgpt

import "time"

// Response represents a ChatGPT response
type Response struct {
	Success  bool       `json:"success"`
	Response ChatGPTMsg `json:"response,omitempty"`
	Metadata Metadata   `json:"metadata,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// ChatGPTMsg represents a ChatGPT message
type ChatGPTMsg struct {
	Text      string    `json:"text"`
	HTML      string    `json:"html,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	TurnIndex int       `json:"turn_index,omitempty"`
}

// Metadata contains additional information about the response
type Metadata struct {
	DocumentPath  string        `json:"document_path,omitempty"`
	Duration      time.Duration `json:"duration_ms"`
	ScreenshotPath string       `json:"screenshot_path,omitempty"`
	ToolCalls     int           `json:"tool_calls,omitempty"`
}
