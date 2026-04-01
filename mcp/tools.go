package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ChromeDevTools provides high-level methods for chrome-devtools-mcp tools
type ChromeDevTools struct {
	client    *Client
	sessionID string // Optional: session ID for multi-tab support
}

// NewChromeDevTools creates a new ChromeDevTools helper
func NewChromeDevTools(client *Client) *ChromeDevTools {
	return &ChromeDevTools{client: client}
}

// WithSessionID returns a new ChromeDevTools instance with the specified session ID
func (cdt *ChromeDevTools) WithSessionID(sessionID string) *ChromeDevTools {
	return &ChromeDevTools{
		client:    cdt.client,
		sessionID: sessionID,
	}
}

// buildArgs builds arguments with sessionId if set
func (cdt *ChromeDevTools) buildArgs(args map[string]interface{}) map[string]interface{} {
	if cdt.sessionID != "" {
		// Add sessionId to arguments for multi-tab support
		args["sessionId"] = cdt.sessionID
	}
	return args
}

// NavigatePage navigates to the specified URL
func (cdt *ChromeDevTools) NavigatePage(ctx context.Context, url string) error {
	_, err := cdt.client.CallTool(ctx, "navigate_page", map[string]interface{}{
		"url": url,
	})
	return err
}

// NewPage opens a new browser tab with the specified URL
// Returns the targetId if available for multi-tab operations
func (cdt *ChromeDevTools) NewPage(ctx context.Context, url string) (string, error) {
	result, err := cdt.client.CallTool(ctx, "new_page", map[string]interface{}{
		"url": url,
	})
	if err != nil {
		return "", err
	}

	// Try to extract targetId from result
	if len(result.Content) > 0 {
		if c, ok := result.Content[0].(mcp.TextContent); ok {
			if cfgDebug {
				log.Printf("[DEBUG] NewPage result: %s", c.Text)
			}
		}
	}

	// Return empty targetId for now
	// In a real implementation, we would extract the actual targetId
	return "", nil
}

// WaitFor waits for specified text to appear on the page
func (cdt *ChromeDevTools) WaitFor(ctx context.Context, texts []string, timeout int) error {
	args := map[string]interface{}{
		"text": texts,
	}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	_, err := cdt.client.CallTool(ctx, "wait_for", args)
	return err
}

// TakeSnapshot captures the current DOM state and returns the text representation
func (cdt *ChromeDevTools) TakeSnapshot(ctx context.Context, verbose bool) (string, error) {
	result, err := cdt.client.CallTool(ctx, "take_snapshot", map[string]interface{}{
		"verbose": verbose,
	})
	if err != nil {
		return "", err
	}

	// Extract content from the result
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty snapshot result")
	}

	content := result.Content[0]

	// Handle different content types
	switch c := content.(type) {
	case mcp.TextContent:
		// Return the text directly
		return c.Text, nil
	case mcp.EmbeddedResource:
		// Handle different resource contents
		switch rc := c.Resource.(type) {
		case mcp.TextResourceContents:
			return rc.Text, nil
		default:
			return "", fmt.Errorf("unexpected resource content type: %T", rc)
		}
	default:
		return "", fmt.Errorf("unexpected content type: %T", c)
	}
}

// Click clicks on an element by its UID
func (cdt *ChromeDevTools) Click(ctx context.Context, uid string) error {
	_, err := cdt.client.CallTool(ctx, "click", map[string]interface{}{
		"uid": uid,
	})
	return err
}

// TypeText types text into the currently focused element
func (cdt *ChromeDevTools) TypeText(ctx context.Context, text string) error {
	_, err := cdt.client.CallTool(ctx, "type_text", map[string]interface{}{
		"text": text,
	})
	return err
}

// Fill fills an input field with text
func (cdt *ChromeDevTools) Fill(ctx context.Context, uid string, value string) (string, error) {
	result, err := cdt.client.CallTool(ctx, "fill", map[string]interface{}{
		"uid":   uid,
		"value": value,
	})
	if err != nil {
		return "", err
	}

	// Return the first content as text if available
	if len(result.Content) > 0 {
		if c, ok := result.Content[0].(mcp.TextContent); ok {
			return c.Text, nil
		}
	}

	return "", nil
}

// PressKey presses a key (e.g., "Enter", "Escape")
func (cdt *ChromeDevTools) PressKey(ctx context.Context, key string) error {
	_, err := cdt.client.CallTool(ctx, "press_key", map[string]interface{}{
		"key": key,
	})
	return err
}

// EvaluateScript executes JavaScript in the page context
func (cdt *ChromeDevTools) EvaluateScript(ctx context.Context, script string) (interface{}, error) {
	result, err := cdt.client.CallTool(ctx, "evaluate_script", map[string]interface{}{
		"function": script,
	})
	if err != nil {
		return nil, err
	}

	// Extract content from the result
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty evaluate_script result")
	}

	content := result.Content[0]

	// Handle different content types
	switch c := content.(type) {
	case mcp.TextContent:
		if cfgDebug {
			log.Printf("[DEBUG] EvaluateScript: TextContent received, length: %d", len(c.Text))
			log.Printf("[DEBUG] EvaluateScript: Raw text (full content): %s", c.Text)
		}

		// Extract JSON from markdown code block if present
		jsonText := extractJSONFromMarkdown(c.Text)
		if jsonText == "" {
			jsonText = c.Text
		}

		// Parse JSON from text
		var jsResult interface{}
		if err := json.Unmarshal([]byte(jsonText), &jsResult); err != nil {
			if cfgDebug {
				log.Printf("[DEBUG] EvaluateScript: Failed to parse as JSON: %v", err)
				log.Printf("[DEBUG] EvaluateScript: Text starts with: %s", truncateString(jsonText, 100))
			}
			return nil, fmt.Errorf("failed to parse script result text: %w", err)
		}
		return jsResult, nil
	case mcp.EmbeddedResource:
		if cfgDebug {
			log.Printf("[DEBUG] EvaluateScript: EmbeddedResource received, type: %T", c.Resource)
		}
		// Handle different resource contents
		switch rc := c.Resource.(type) {
		case mcp.TextResourceContents:
			if cfgDebug {
				log.Printf("[DEBUG] EvaluateScript: TextResourceContents, length: %d", len(rc.Text))
				log.Printf("[DEBUG] EvaluateScript: Raw text (full content): %s", rc.Text)
			}

			// Extract JSON from markdown code block if present
			jsonText := extractJSONFromMarkdown(rc.Text)
			if jsonText == "" {
				jsonText = rc.Text
			}

			var jsResult interface{}
			if err := json.Unmarshal([]byte(jsonText), &jsResult); err != nil {
				if cfgDebug {
					log.Printf("[DEBUG] EvaluateScript: Failed to parse as JSON: %v", err)
				}
				return nil, fmt.Errorf("failed to parse script result text: %w", err)
			}
			return jsResult, nil
		case mcp.BlobResourceContents:
			if cfgDebug {
				log.Printf("[DEBUG] EvaluateScript: BlobResourceContents, blob length: %d", len(rc.Blob))
			}
			var jsResult interface{}
			// Decode base64 blob manually
			data, err := base64.StdEncoding.DecodeString(rc.Blob)
			if err != nil {
				return nil, fmt.Errorf("failed to decode script result blob: %w", err)
			}
			if cfgDebug {
				log.Printf("[DEBUG] EvaluateScript: Decoded blob data (full content): %s", string(data))
			}

			// Extract JSON from markdown code block if present
			jsonText := extractJSONFromMarkdown(string(data))
			if jsonText == "" {
				jsonText = string(data)
			}

			if err := json.Unmarshal([]byte(jsonText), &jsResult); err != nil {
				if cfgDebug {
					log.Printf("[DEBUG] EvaluateScript: Failed to parse blob as JSON: %v", err)
				}
				return nil, fmt.Errorf("failed to parse script result blob data: %w", err)
			}
			return jsResult, nil
		default:
			return nil, fmt.Errorf("unexpected resource content type: %T", rc)
		}
	default:
		return nil, fmt.Errorf("unexpected content type: %T", c)
	}
}

// extractJSONFromMarkdown extracts JSON content from markdown code blocks
// Handles formats like:
// ```json
// {...}
// ```
func extractJSONFromMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	var jsonLines []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for code block start
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				// Starting code block
				inCodeBlock = true
				continue
			} else {
				// Ending code block
				inCodeBlock = false
				break
			}
		}

		// If we're in a code block, collect the content
		if inCodeBlock {
			jsonLines = append(jsonLines, line)
		}
	}

	if len(jsonLines) > 0 {
		result := strings.Join(jsonLines, "\n")
		if cfgDebug {
			log.Printf("[DEBUG] Extracted JSON from markdown, length: %d", len(result))
		}
		return result
	}

	return ""
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TakeScreenshot takes a screenshot and saves it to the specified path
func (cdt *ChromeDevTools) TakeScreenshot(ctx context.Context, path string) error {
	_, err := cdt.client.CallTool(ctx, "take_screenshot", map[string]interface{}{
		"path": path,
	})
	return err
}

// ListPages lists all open browser pages
func (cdt *ChromeDevTools) ListPages(ctx context.Context) ([]map[string]interface{}, error) {
	result, err := cdt.client.CallTool(ctx, "list_pages", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	// Extract content from the result
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty list_pages result")
	}

	content := result.Content[0]

	var text string
	// Handle different content types
	switch c := content.(type) {
	case mcp.TextContent:
		text = c.Text
	case mcp.EmbeddedResource:
		switch rc := c.Resource.(type) {
		case mcp.TextResourceContents:
			text = rc.Text
		case mcp.BlobResourceContents:
			data, err := base64.StdEncoding.DecodeString(rc.Blob)
			if err != nil {
				return nil, fmt.Errorf("failed to decode pages blob: %w", err)
			}
			text = string(data)
		default:
			return nil, fmt.Errorf("unexpected resource content type: %T", rc)
		}
	default:
		return nil, fmt.Errorf("unexpected content type: %T", c)
	}

	if cfgDebug {
		log.Printf("[DEBUG] ListPages: Raw text: %s", text)
	}

	// Parse the text format:
	// ## Pages
	// 1: about:blank
	// 2: about:blank [selected]
	pages, err := parsePagesText(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pages text: %w", err)
	}

	if cfgDebug {
		log.Printf("[DEBUG] ListPages: Parsed %d pages", len(pages))
	}

	return pages, nil
}

// parsePagesText parses the pages list text format
func parsePagesText(text string) ([]map[string]interface{}, error) {
	lines := strings.Split(text, "\n")
	var pages []map[string]interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip header line
		if strings.HasPrefix(line, "##") {
			continue
		}

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse line: "1: about:blank [selected]"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		// Extract page ID
		pageIDStr := strings.TrimSpace(parts[0])
		var pageID int
		_, err := fmt.Sscanf(pageIDStr, "%d", &pageID)
		if err != nil {
			continue
		}

		// Extract URL and metadata
		rest := strings.TrimSpace(parts[1])
		selected := false

		// Check if [selected] is present
		if strings.Contains(rest, "[selected]") {
			selected = true
			rest = strings.ReplaceAll(rest, "[selected]", "")
			rest = strings.TrimSpace(rest)
		}

		page := map[string]interface{}{
			"pageId":   float64(pageID), // JSON numbers are float64
			"url":      rest,
			"selected": selected,
		}

		pages = append(pages, page)
	}

	return pages, nil
}

var cfgDebug = false

// SetDebug sets the debug flag for tools package
func SetDebug(debug bool) {
	cfgDebug = debug
}

// SelectPage selects a page as the context for future tool calls
func (cdt *ChromeDevTools) SelectPage(ctx context.Context, pageId int, bringToFront bool) error {
	if cfgDebug {
		log.Printf("[DEBUG] SelectPage: Calling select_page with pageId=%d, bringToFront=%v", pageId, bringToFront)
	}

	args := map[string]interface{}{
		"pageId": pageId,
	}
	if bringToFront {
		args["bringToFront"] = true
	}

	result, err := cdt.client.CallTool(ctx, "select_page", args)
	if err != nil {
		if cfgDebug {
			log.Printf("[DEBUG] SelectPage: Error: %v", err)
		}
		return err
	}

	if cfgDebug && len(result.Content) > 0 {
		if c, ok := result.Content[0].(mcp.TextContent); ok {
			log.Printf("[DEBUG] SelectPage: Result: %s", c.Text)
		}
	}

	return nil
}

// ClosePage closes the specified page by its ID to free memory
func (cdt *ChromeDevTools) ClosePage(ctx context.Context, pageId int) error {
	if cfgDebug {
		log.Printf("[DEBUG] ClosePage: Closing page %d", pageId)
	}

	_, err := cdt.client.CallTool(ctx, "close_page", map[string]interface{}{
		"pageId": pageId,
	})
	if err != nil {
		if cfgDebug {
			log.Printf("[DEBUG] ClosePage: Error closing page %d: %v", pageId, err)
		}
		return err
	}

	if cfgDebug {
		log.Printf("[DEBUG] ClosePage: Page %d closed successfully", pageId)
	}

	return nil
}
