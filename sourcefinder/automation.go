package sourcefinder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Agent handles SourceFinder automation
type Agent struct {
	client *Client
	config *Config
}

// NewAgent creates a new SourceFinder automation agent
func NewAgent(baseURL, apiKey string, timeout time.Duration, debug bool) *Agent {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: timeout,
		Debug:   debug,
	}

	return &Agent{
		client: NewClient(config),
		config: config,
	}
}

// NewAgentWithConfig creates a new SourceFinder automation agent with custom parameters
func NewAgentWithConfig(baseURL, apiKey string, timeout time.Duration, debug bool, engines []string, maxResults int, model string) *Agent {
	config := &Config{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Timeout:    timeout,
		Debug:      debug,
		Engines:    engines,
		MaxResults: maxResults,
		Model:      model,
	}

	return &Agent{
		client: NewClient(config),
		config: config,
	}
}

// InputAndSubmitOnly submits a fact-checking job to SourceFinder
// This implements the same interface as ChatGPT and Grok agents
func (a *Agent) InputAndSubmitOnly(ctx context.Context, documentContent string) error {
	// SourceFinder doesn't use the "input and submit only" pattern
	// It directly processes the entire prompt in one call
	// This method is a no-op for SourceFinder
	if a.config.Debug {
		log.Printf("[SourceFinder] InputAndSubmitOnly called (no-op for SourceFinder)")
	}
	return nil
}

// WaitForResponse waits for and returns the SourceFinder response
// This implements the same interface as ChatGPT and Grok agents
func (a *Agent) WaitForResponse(ctx context.Context) (*Response, error) {
	// For SourceFinder, the actual processing happens in ProcessDocument
	// This is a placeholder to maintain interface compatibility
	return nil, fmt.Errorf("use ProcessDocument for SourceFinder instead")
}

// ProcessDocument sends a document to SourceFinder and returns the response
// This is the main method for SourceFinder processing
func (a *Agent) ProcessDocument(ctx context.Context, documentContent string) (*Response, error) {
	startTime := time.Now()

	if a.config.Debug {
		log.Printf("[SourceFinder] Processing document (%d chars)", len(documentContent))
	}

	// Parse the document content to extract title and content
	// The template format is: "${input}" where input is the claim/title
	title, content := a.parseDocumentContent(documentContent)

	if a.config.Debug {
		log.Printf("[SourceFinder] Title: '%s'", title)
		log.Printf("[SourceFinder] Content length: %d", len(content))
	}

	// Submit fact-checking job
	jobResp, err := a.client.ProcessFactCheck(ctx, title, content, nil)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to process fact check: %v", err),
		}, fmt.Errorf("process fact check failed: %w", err)
	}

	// Format result as JSON
	jsonResult, err := FormatAsJSON(jobResp)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to format result: %v", err),
		}, fmt.Errorf("format result failed: %w", err)
	}

	duration := time.Since(startTime)

	if a.config.Debug {
		log.Printf("[SourceFinder] Processing completed in %v", duration)
		log.Printf("[SourceFinder] Result length: %d chars", len(jsonResult))
	}

	return &Response{
		Success: true,
		Response: SourceMsg{
			Text: jsonResult,
		},
		Metadata: Metadata{
			Duration:  duration,
			ToolCalls: 1,
		},
	}, nil
}

// parseDocumentContent parses the document content to extract title and content
// The template format includes the user's claim which we use as the title
func (a *Agent) parseDocumentContent(documentContent string) (title, content string) {
	lines := strings.Split(documentContent, "\n")

	// Find the claim in the document
	// Typically the last line or the line after a specific marker
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "```") && !strings.HasPrefix(line, "#") {
			// Use this as the title
			title = line
			break
		}
	}

	// If no title found, use a default
	if title == "" {
		title = "Fact Check Request"
	}

	// Use the entire document as content
	content = documentContent

	return title, content
}

// ProcessFromInput is a convenience method that processes a user input directly
// This is used by main.go for batch processing
func (a *Agent) ProcessFromInput(ctx context.Context, inputText string) (*Response, error) {
	if a.config.Debug {
		log.Printf("[SourceFinder] Processing input: '%s'", truncateString(inputText, 100))
	}

	// Create a simple document from the input
	documentContent := fmt.Sprintf("Fact Check Request:\n\n%s", inputText)

	return a.ProcessDocument(ctx, documentContent)
}

// ProcessFromStructuredInput processes a structured input with title, content, and source URLs
// This is the recommended method for processing FactCheckInput
func (a *Agent) ProcessFromStructuredInput(ctx context.Context, title, content string, sourceURLs []string) (*Response, error) {
	startTime := time.Now()

	if a.config.Debug {
		log.Printf("[SourceFinder] Processing structured input:")
		log.Printf("[SourceFinder]   Title: '%s'", truncateString(title, 100))
		log.Printf("[SourceFinder]   Content length: %d", len(content))
		log.Printf("[SourceFinder]   Source URLs: %d", len(sourceURLs))
	}

	// Submit fact-checking job
	jobResp, err := a.client.ProcessFactCheck(ctx, title, content, sourceURLs)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to process fact check: %v", err),
		}, fmt.Errorf("process fact check failed: %w", err)
	}

	// Extract job ID from response
	jobID := jobResp.ID

	// Format result as JSON
	jsonResult, err := FormatAsJSON(jobResp)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to format result: %v", err),
		}, fmt.Errorf("format result failed: %w", err)
	}

	duration := time.Since(startTime)

	if a.config.Debug {
		log.Printf("[SourceFinder] Processing completed in %v", duration)
		log.Printf("[SourceFinder] Result length: %d chars", len(jsonResult))
	}

	return &Response{
		Success: true,
		Response: SourceMsg{
			Text: jsonResult,
		},
		Metadata: Metadata{
			Duration:  duration,
			ToolCalls: 1,
			JobID:     jobID,
		},
	}, nil
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GetResponseAsMap returns the response as a map for easier access
func (a *Agent) GetResponseAsMap(response *Response) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Response.Text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}
	return result, nil
}
