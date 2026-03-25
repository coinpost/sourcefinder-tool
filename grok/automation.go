package grok

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/coinpost/sourcefinder-tool/mcp"
)

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const (
	// GrokURL is the base URL for Grok
	GrokURL = "https://grok.com/"
)

// Agent handles Grok automation using chrome-devtools-mcp
type Agent struct {
	cdt       *mcp.ChromeDevTools
	selectors *Selectors
	timeout   time.Duration
	debug     bool
}

// NewAgent creates a new Grok automation agent
func NewAgent(cdt *mcp.ChromeDevTools, timeout time.Duration, debug bool) *Agent {
	return &Agent{
		cdt:       cdt,
		selectors: NewSelectors(),
		timeout:   timeout,
		debug:     debug,
	}
}

// ProcessDocument sends a document to Grok and returns the response
func (a *Agent) ProcessDocument(ctx context.Context, documentContent string) (*Response, error) {
	startTime := time.Now()

	// Step 1: Navigate to Grok
	if a.debug {
		log.Printf("Navigating to %s", GrokURL)
	}
	if err := a.cdt.NavigatePage(ctx, GrokURL); err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to navigate: %v", err),
		}, fmt.Errorf("navigation failed: %w", err)
	}

	// Step 2: Wait for page to load completely
	if a.debug {
		log.Printf("Waiting for page to load...")
	}
	if err := a.cdt.WaitFor(ctx, []string{"Message Grok", "Send message", "Send"}, 20000); err != nil {
		// If wait fails, try taking snapshot to see what's there
		if a.debug {
			log.Printf("Wait failed, trying alternative approach...")
		}
		// Give it a moment anyway
		time.Sleep(2 * time.Second)
	}

	// Additional wait to ensure page is fully ready
	if a.debug {
		log.Printf("Waiting additional 2 seconds for page to stabilize...")
	}
	time.Sleep(2 * time.Second)

	// Step 3: Take snapshot to find input element
	if a.debug {
		log.Printf("Taking snapshot to find input element...")
	}
	snapshot, err := a.cdt.TakeSnapshot(ctx, true)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to take snapshot: %v", err),
		}, fmt.Errorf("snapshot failed: %w", err)
	}

	// Step 4: Find input element UID
	inputUID, err := a.selectors.FindInputUID(snapshot)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to find input element: %v", err),
		}, fmt.Errorf("input element not found: %w", err)
	}
	if a.debug {
		log.Printf("Found input element with UID: %s", inputUID)
	}

	// Step 5: Click input to focus
	if a.debug {
		log.Printf("Clicking input element...")
	}
	if err := a.cdt.Click(ctx, inputUID); err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to click input: %v", err),
		}, fmt.Errorf("click failed: %w", err)
	}

	// Small delay after clicking
	time.Sleep(500 * time.Millisecond)

	// Step 6: Clear input field before typing
	if a.debug {
		log.Printf("Clearing input field...")
	}
	// Select all and delete to clear the field
	if err := a.cdt.PressKey(ctx, "Control+a"); err != nil {
		if a.debug {
			log.Printf("Warning: Failed to select all (Ctrl+a): %v", err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if err := a.cdt.PressKey(ctx, "Delete"); err != nil {
		if a.debug {
			log.Printf("Warning: Failed to press Delete: %v", err)
		}
	}
	time.Sleep(200 * time.Millisecond)

	// Step 7: Type document content line by line using Shift+Enter for newlines
	if a.debug {
		log.Printf("Typing document content (%d chars) line by line...", len(documentContent))
	}

	// Split content into lines
	lines := strings.Split(documentContent, "\n")
	if a.debug {
		log.Printf("Document has %d lines", len(lines))
	}

	for i, line := range lines {
		// Type the line content
		if line != "" {
			if err := a.cdt.TypeText(ctx, line); err != nil {
				return &Response{
					Success: false,
					Error:   fmt.Sprintf("failed to type line %d: %v", i+1, err),
				}, fmt.Errorf("type text failed: %w", err)
			}
			if a.debug {
				log.Printf("Typed line %d/%d (%d chars)", i+1, len(lines), len(line))
			}
			// Small delay between characters to ensure typing completes
			time.Sleep(time.Duration(len(line)*10+100) * time.Millisecond)
		}

		// If not the last line, press Shift+Enter to create a new line
		if i < len(lines)-1 {
			if a.debug {
				log.Printf("Pressing Shift+Enter for new line...")
			}
			if err := a.cdt.PressKey(ctx, "Shift+Enter"); err != nil {
				if a.debug {
					log.Printf("Warning: Failed to press Shift+Enter: %v", err)
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	if a.debug {
		log.Printf("Finished typing all lines")
	}

	// Wait a moment to ensure all typing is complete
	time.Sleep(500 * time.Millisecond)

	// Step 8: Wait before sending (1-3 seconds random delay)
	randomDelay := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
	if a.debug {
		log.Printf("Waiting %v before sending...", randomDelay)
	}
	time.Sleep(randomDelay)

	// Step 9: Submit with Enter key
	if a.debug {
		log.Printf("Submitting with Enter key...")
	}
	if err := a.cdt.PressKey(ctx, "Enter"); err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to press Enter: %v", err),
		}, fmt.Errorf("press key failed: %w", err)
	}

	// Step 10: Poll for response element to appear
	if a.debug {
		log.Printf("Phase 1: Polling for response element...")
	}

	pollInterval := 3 * time.Second
	deadline := time.Now().Add(a.timeout)
	elementFound := false

	for time.Now().Before(deadline) {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled",
			}, ctx.Err()
		default:
		}

		// Use CheckResponseReadyScript to poll for response element
		checkScript := a.selectors.CheckResponseReadyScript()
		result, err := a.cdt.EvaluateScript(ctx, checkScript)
		if err != nil {
			if a.debug {
				log.Printf("Warning: check script failed: %v", err)
			}
		} else {
			if resultMap, ok := result.(map[string]interface{}); ok {
				// Check if response element was found
				if found, ok := resultMap["found"].(bool); ok && found {
					if a.debug {
						totalDivs := 0
						if td, ok := resultMap["totalDivs"].(float64); ok {
							totalDivs = int(td)
						}
						log.Printf("Response element found! (totalDivs: %d)", totalDivs)
					}
					elementFound = true
					break
				}
			}
		}

		// Wait before next poll
		if a.debug {
			log.Printf("Element not found yet, waiting %v...", pollInterval)
		}
		select {
		case <-time.After(pollInterval):
			// Continue polling
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled during wait",
			}, ctx.Err()
		}
	}

	if !elementFound {
		return &Response{
			Success: false,
			Error:   "timeout waiting for response element to appear",
		}, fmt.Errorf("timeout: response element not found")
	}

	// Step 11: Poll and try to parse JSON until success or timeout
	if a.debug {
		log.Printf("Phase 2: Polling for valid JSON content...")
	}

	jsonPollInterval := 2 * time.Second
	attempts := 0

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled",
			}, ctx.Err()
		default:
		}

		attempts++

		// Try to extract and parse JSON
		scriptResult, err := a.cdt.EvaluateScript(ctx, a.selectors.GetResponseScript())
		if err != nil {
			if a.debug {
				log.Printf("Attempt %d: Extract script failed: %v", attempts, err)
			}
		} else {
			if resultMap, ok := scriptResult.(map[string]interface{}); ok {
				// Check if we got an error
				if errMsg, hasError := resultMap["error"]; hasError {
					if a.debug {
						log.Printf("Attempt %d: Extraction returned error: %v", attempts, errMsg)
					}
				} else {
					// We got a result, check if it has text
					if text, ok := resultMap["text"].(string); ok && len(text) > 0 {
						// Try to parse as JSON to verify it's valid
						var jsonData interface{}
						if err := json.Unmarshal([]byte(text), &jsonData); err == nil {
							// Successfully parsed as JSON!
							if a.debug {
								method := resultMap["method"]
								totalJsonBlocks := 0
								if tjb, ok := resultMap["totalJsonBlocks"].(float64); ok {
									totalJsonBlocks = int(tjb)
								}
								log.Printf("✓ Successfully parsed JSON (method: %v, blocks: %d, length: %d chars)",
									method, totalJsonBlocks, len(text))
							}

							// Parse the response message
							responseMsg, err := a.selectors.ParseResponseResult(scriptResult)
							if err != nil {
								return &Response{
									Success: false,
									Error:   fmt.Sprintf("failed to parse response: %v", err),
								}, fmt.Errorf("parse response failed: %w", err)
							}

							duration := time.Since(startTime)
							return &Response{
								Success:  true,
								Response: *responseMsg,
								Metadata: Metadata{
									Duration:  duration,
									ToolCalls: 10,
								},
							}, nil
						} else {
							// Not valid JSON yet, continue waiting
							if a.debug {
								log.Printf("Attempt %d: Content found but JSON parse failed: %v (length: %d)",
									attempts, err, len(text))
								if len(text) < 200 {
									log.Printf("Content preview: %s", text)
								} else {
									log.Printf("Content preview: %s...", text[:200])
								}
							}
						}
					} else {
						if a.debug {
							log.Printf("Attempt %d: No text content in result", attempts)
						}
					}
				}
			}
		}

		if a.debug {
			log.Printf("JSON not ready yet, waiting %v before retry...", jsonPollInterval)
		}
		select {
		case <-time.After(jsonPollInterval):
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled during JSON polling",
			}, ctx.Err()
		}
	}

	return &Response{
		Success: false,
		Error:   fmt.Sprintf("timeout after %d attempts waiting for valid JSON", attempts),
	}, fmt.Errorf("timeout: failed to parse valid JSON after %d attempts", attempts)
}

// scrapeResponse extracts the Grok response from the page
func (a *Agent) scrapeResponse(ctx context.Context) (*GrokMsg, error) {
	if a.debug {
		log.Printf("[DEBUG] scrapeResponse: Starting...")
	}

	// Execute JavaScript to extract response
	if a.debug {
		log.Printf("[DEBUG] scrapeResponse: Executing GetResponseScript...")
	}
	result, err := a.cdt.EvaluateScript(ctx, a.selectors.GetResponseScript())
	if err != nil {
		if a.debug {
			log.Printf("[DEBUG] scrapeResponse: EvaluateScript failed: %v", err)
		}
		return nil, fmt.Errorf("evaluate script failed: %w", err)
	}

	if a.debug {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		log.Printf("[DEBUG] scrapeResponse: Script result: %s", string(resultJSON))
	}

	// Parse the result
	if a.debug {
		log.Printf("[DEBUG] scrapeResponse: Parsing result...")
	}
	msg, err := a.selectors.ParseResponseResult(result)
	if err != nil {
		if a.debug {
			log.Printf("[DEBUG] scrapeResponse: ParseResponseResult failed: %v", err)
		}
		return nil, fmt.Errorf("parse response result failed: %w", err)
	}

	if a.debug {
		log.Printf("[DEBUG] scrapeResponse: Successfully parsed message, text length: %d", len(msg.Text))
		if len(msg.Text) > 0 {
			preview := msg.Text
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			log.Printf("[DEBUG] scrapeResponse: Text preview: %s", preview)
		}
	}

	return msg, nil
}

// NavigateAndSubmitWithNewTab opens a new tab via chrome-devtools-mcp and processes the input
func (a *Agent) NavigateAndSubmitWithNewTab(ctx context.Context, documentContent string) error {
	// Step 1: Open new page with Grok URL
	if a.debug {
		log.Printf("Opening new tab with %s...", GrokURL)
	}

	if _, err := a.cdt.NewPage(ctx, GrokURL); err != nil {
		return fmt.Errorf("failed to open new page: %w", err)
	}

	// Wait for page to load
	if a.debug {
		log.Printf("Waiting for page to load...")
	}
	time.Sleep(2 * time.Second)

	// Step 2: Continue with input and submit (skip navigation since we already navigated)
	return a.InputAndSubmitOnly(ctx, documentContent)
}

// InputAndSubmitOnly only does input and submit, assuming already on Grok page
func (a *Agent) InputAndSubmitOnly(ctx context.Context, documentContent string) error {
	if a.debug {
		log.Printf("[Grok] InputAndSubmitOnly called with %d chars of content", len(documentContent))
	}

	// Use the new smart finder
	smartFinder := NewSmartInputFinder(a.cdt, a.debug)

	// Wait for input to be ready
	if a.debug {
		log.Printf("[Grok] Waiting for input to be ready...")
	}
	if err := smartFinder.WaitForInputReady(ctx, 10*time.Second); err != nil {
		if a.debug {
			log.Printf("[Grok] WARNING: WaitForInputReady failed: %v", err)
			log.Printf("[Grok] Continuing anyway with 2 second delay...")
		}
		time.Sleep(2 * time.Second)
	} else {
		if a.debug {
			log.Printf("[Grok] Input is ready!")
		}
	}

	// Try the smart finder first
	if a.debug {
		log.Printf("[Grok] Attempting smart input fill...")
	}
	if err := smartFinder.FindAndFillInput(ctx, documentContent); err != nil {
		if a.debug {
			log.Printf("[Grok] ERROR: Smart finder failed: %v", err)
			log.Printf("[Grok] Will NOT fall back to legacy method - smart finder should work")
		}
		// Return the error instead of falling back
		// This helps us see the real issue
		return fmt.Errorf("smart finder failed: %w", err)
	}

	if a.debug {
		log.Printf("[Grok] ✓ Smart finder successfully filled input")
	}

	// Wait a moment after filling
	time.Sleep(500 * time.Millisecond)

	// Submit using smart finder
	if a.debug {
		log.Printf("[Grok] Attempting smart submit...")
	}
	if err := smartFinder.SubmitInput(ctx); err != nil {
		if a.debug {
			log.Printf("[Grok] ERROR: Smart submit failed: %v", err)
		}
		return fmt.Errorf("smart submit failed: %w", err)
	}

	if a.debug {
		log.Printf("[Grok] ✓ Successfully submitted input using smart finder")
	}

	return nil
}

// inputAndSubmitLegacy is the legacy input method (original implementation)
func (a *Agent) inputAndSubmitLegacy(ctx context.Context, documentContent string) error {
	// Wait for page to stabilize
	if a.debug {
		log.Printf("Waiting additional 2 seconds for page to stabilize...")
	}
	time.Sleep(2 * time.Second)

	// Try JavaScript-based input first (more reliable for Grok)
	if a.debug {
		log.Printf("Attempting JavaScript-based input method...")
	}

	// Escape the document content for JavaScript
	escapedContent := strings.ReplaceAll(documentContent, "\\", "\\\\")
	escapedContent = strings.ReplaceAll(escapedContent, "\"", "\\\"")
	escapedContent = strings.ReplaceAll(escapedContent, "\n", "\\n")
	escapedContent = strings.ReplaceAll(escapedContent, "\r", "\\r")
	escapedContent = strings.ReplaceAll(escapedContent, "\t", "\\t")

	// Try multiple selectors to find and interact with the input
	inputScript := fmt.Sprintf(`() => {
		// First try to find the Grok-specific querybar and then find contenteditable inside
		const querybar = document.querySelector('div.querybar') || document.querySelector('[class*="querybar"]');
		if (querybar) {
			// Find contenteditable div inside querybar
			const editableDiv = querybar.querySelector('[contenteditable="true"]');
			if (editableDiv) {
				// Check if element is visible
				const rect = editableDiv.getBoundingClientRect();
				if (rect.width > 0 && rect.height > 0) {
					// Focus the element first
					editableDiv.focus();

					// Set text content
					editableDiv.textContent = %q;

					// Trigger events to notify Grok
					editableDiv.dispatchEvent(new Event('input', { bubbles: true }));
					editableDiv.dispatchEvent(new Event('change', { bubbles: true }));
					editableDiv.dispatchEvent(new Event('keyup', { bubbles: true }));

					return { success: true, selector: 'div.querybar [contenteditable="true"]', method: 'querybar_contenteditable', length: %d };
				}
			}
		}

		// Fallback to other selectors
		const selectors = [
			// Common contenteditable selectors
			'[contenteditable="true"]',
			'div[contenteditable]',
			'textarea',
			'input[type="text"]',
			// Specific Grok selectors (may need adjustment)
			'div[data-testid="prompt-textarea"]',
			'textarea[data-testid="prompt-textarea"]',
			'div[class*="prompt"]',
			'div[class*="input"]',
			'textarea[class*="prompt"]',
		];

		for (const selector of selectors) {
			const element = document.querySelector(selector);
			if (element) {
				// Check if element is visible and editable
				const rect = element.getBoundingClientRect();
				if (rect.width > 0 && rect.height > 0) {
					// For contenteditable divs
					if (element.contentEditable === 'true') {
						element.focus();
						element.textContent = %q;
						element.dispatchEvent(new Event('input', { bubbles: true }));
						element.dispatchEvent(new Event('change', { bubbles: true }));
						return { success: true, selector: selector, method: 'contenteditable', length: %d };
					}
					// For textarea/input
					else if (element.value !== undefined) {
						element.focus();
						element.value = %q;
						element.dispatchEvent(new Event('input', { bubbles: true }));
						element.dispatchEvent(new Event('change', { bubbles: true }));
						return { success: true, selector: selector, method: 'value', length: %d };
					}
				}
			}
		}

		return { error: "No suitable input element found" };
	}`, escapedContent, len(documentContent), escapedContent, len(documentContent), escapedContent, len(documentContent))

	result, err := a.cdt.EvaluateScript(ctx, inputScript)
	if err != nil {
		if a.debug {
			log.Printf("[DEBUG] JavaScript input failed: %v, trying fallback method...", err)
		}
	} else {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if success, ok := resultMap["success"].(bool); ok && success {
				if a.debug {
					resultJSON, _ := json.MarshalIndent(result, "", "  ")
					log.Printf("JavaScript input succeeded: %s", string(resultJSON))
				}
			} else if errMsg, ok := resultMap["error"].(string); ok {
				if a.debug {
					log.Printf("[DEBUG] JavaScript input returned error: %s, trying fallback...", errMsg)
				}
			}
		}
	}

	// Fallback to snapshot-based method if JavaScript failed
	if a.debug {
		log.Printf("Trying snapshot-based input method...")
	}

	snapshot, err := a.cdt.TakeSnapshot(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to take snapshot: %w", err)
	}

	// Find input element UID
	inputUID, err := a.selectors.FindInputUID(snapshot)
	if err != nil {
		// Print snapshot for debugging
		if a.debug {
			log.Printf("[DEBUG] Snapshot content (first 3000 chars):\n%s", truncateString(snapshot, 3000))
			log.Printf("[DEBUG] Snapshot content (last 2000 chars):\n%s", snapshot[max(0, len(snapshot)-2000):])
			log.Printf("[DEBUG] %s", a.selectors.FindQuerybarLocation(snapshot))
			log.Printf("[DEBUG] Total snapshot size: %d chars, %d lines", len(snapshot), len(strings.Split(snapshot, "\n")))
		}
		return fmt.Errorf("failed to find input element: %w", err)
	}

	if a.debug {
		log.Printf("Found input element with UID: %s", inputUID)
	}

	// Click input to focus
	if a.debug {
		log.Printf("Clicking input element...")
	}
	if err := a.cdt.Click(ctx, inputUID); err != nil {
		return fmt.Errorf("failed to click input: %w", err)
	}

	// Small delay after clicking
	time.Sleep(500 * time.Millisecond)

	// Clear input field before typing
	if a.debug {
		log.Printf("Clearing input field...")
	}
	if err := a.cdt.PressKey(ctx, "Control+a"); err != nil {
		if a.debug {
			log.Printf("Warning: Failed to select all (Ctrl+a): %v", err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if err := a.cdt.PressKey(ctx, "Delete"); err != nil {
		if a.debug {
			log.Printf("Warning: Failed to press Delete: %v", err)
		}
	}
	time.Sleep(200 * time.Millisecond)

	// Type document content using fill tool
	if a.debug {
		log.Printf("Setting input content (%d chars)...", len(documentContent))
	}

	// Use fill tool instead of JavaScript
	_, err = a.cdt.Fill(ctx, inputUID, documentContent)
	if err != nil {
		return fmt.Errorf("failed to fill input: %w", err)
	}

	if a.debug {
		log.Printf("Input content set successfully")
	}

	// Wait a moment to ensure all typing is complete
	time.Sleep(500 * time.Millisecond)

	// Submit using legacy method
	return a.submitLegacy(ctx)
}

// submitLegacy is the legacy submit method
func (a *Agent) submitLegacy(ctx context.Context) error {
	// Wait before sending (1-3 seconds random delay)
	randomDelay := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
	if a.debug {
		log.Printf("Waiting %v before sending...", randomDelay)
	}
	time.Sleep(randomDelay)

	// Try JavaScript-based submit first
	submitScript := `() => {
		// Try multiple selectors for the send button
		const selectors = [
			'button[aria-label="Send"]',
			'button[aria-label="Send message"]',
			'button[data-testid="send-button"]',
			'button[type="submit"]',
			'button svg',
		];

		for (const selector of selectors) {
			const button = document.querySelector(selector);
			if (button && button.offsetParent !== null) {
				button.click();
				return { success: true, selector: selector };
			}
		}

		// Try pressing Enter as fallback
		document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
		return { success: true, method: 'Enter key' };
	}`

	result, err := a.cdt.EvaluateScript(ctx, submitScript)
	if err != nil {
		if a.debug {
			log.Printf("[DEBUG] JavaScript submit failed: %v, trying snapshot method...", err)
		}
	} else {
		if a.debug {
			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			log.Printf("JavaScript submit result: %s", string(resultJSON))
		}
		// If JavaScript submit succeeded, return
		if resultMap, ok := result.(map[string]interface{}); ok {
			if success, ok := resultMap["success"].(bool); ok && success {
				if a.debug {
					log.Printf("JavaScript submit succeeded")
				}
				return nil
			}
		}
	}

	// Fallback to snapshot-based submit if needed
	snapshot, err := a.cdt.TakeSnapshot(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to take snapshot: %w", err)
	}

	// Find send button UID
	sendButtonUID, err := a.selectors.FindSendButtonUID(snapshot)
	if err != nil {
		return fmt.Errorf("failed to find send button: %w", err)
	}

	if a.debug {
		log.Printf("Found send button with UID: %s", sendButtonUID)
	}

	// Click send button
	if a.debug {
		log.Printf("Clicking send button...")
	}
	if err := a.cdt.Click(ctx, sendButtonUID); err != nil {
		return fmt.Errorf("failed to click send button: %w", err)
	}

	if a.debug {
		log.Printf("Input submitted successfully")
	}

	return nil
}

// WaitForResponse waits for Grok response and extracts result (can be parallel)
func (a *Agent) WaitForResponse(ctx context.Context) (*Response, error) {
	if a.debug {
		log.Printf("Waiting for Grok response...")
	}

	pollInterval := 3 * time.Second
	deadline := time.Now().Add(a.timeout)

	// Phase 1: Wait for response element to appear
	if a.debug {
		log.Printf("Phase 1: Waiting for response element to appear...")
	}

	elementFound := false
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled",
			}, ctx.Err()
		default:
		}

		// Use CheckResponseReadyScript to poll for response element
		checkScript := a.selectors.CheckResponseReadyScript()
		result, err := a.cdt.EvaluateScript(ctx, checkScript)
		if err != nil {
			if a.debug {
				log.Printf("Warning: check script failed: %v", err)
			}
		} else {
			if resultMap, ok := result.(map[string]interface{}); ok {
				// Check if response element was found
				if found, ok := resultMap["found"].(bool); ok && found {
					if a.debug {
						totalDivs := 0
						if td, ok := resultMap["totalDivs"].(float64); ok {
							totalDivs = int(td)
						}
						log.Printf("Response element found! (totalDivs: %d)", totalDivs)
					}
					elementFound = true
					break
				}
			}
		}

		if a.debug {
			log.Printf("Element not found yet, waiting %v...", pollInterval)
		}
		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled during wait",
			}, ctx.Err()
		}
	}

	if !elementFound {
		return &Response{
			Success: false,
			Error:   "timeout waiting for response element to appear",
		}, fmt.Errorf("timeout: response element not found")
	}

	// Phase 2: Poll and try to parse JSON until success or timeout
	if a.debug {
		log.Printf("Phase 2: Polling for valid JSON content...")
	}

	jsonPollInterval := 2 * time.Second // Faster polling for JSON parsing
	attempts := 0

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled",
			}, ctx.Err()
		default:
		}

		attempts++

		// Try to extract and parse JSON
		result, err := a.cdt.EvaluateScript(ctx, a.selectors.GetResponseScript())
		if err != nil {
			if a.debug {
				log.Printf("Attempt %d: Extract script failed: %v", attempts, err)
			}
		} else {
			if resultMap, ok := result.(map[string]interface{}); ok {
				// Check if we got an error
				if errMsg, hasError := resultMap["error"]; hasError {
					if a.debug {
						log.Printf("Attempt %d: Extraction returned error: %v", attempts, errMsg)
					}
				} else {
					// We got a result, check if it has text
					if text, ok := resultMap["text"].(string); ok && len(text) > 0 {
						// Try to parse as JSON to verify it's valid
						var jsonData interface{}
						if err := json.Unmarshal([]byte(text), &jsonData); err == nil {
							// Successfully parsed as JSON!
							if a.debug {
								method := resultMap["method"]
								totalJsonBlocks := 0
								if tjb, ok := resultMap["totalJsonBlocks"].(float64); ok {
									totalJsonBlocks = int(tjb)
								}
								log.Printf("✓ Successfully parsed JSON (method: %v, blocks: %d, length: %d chars)",
									method, totalJsonBlocks, len(text))
							}

							// Parse the response message
							msg, err := a.selectors.ParseResponseResult(result)
							if err != nil {
								return &Response{
									Success: false,
									Error:   fmt.Sprintf("failed to parse response: %v", err),
								}, fmt.Errorf("parse response failed: %w", err)
							}

							return &Response{
								Success:  true,
								Response: *msg,
							}, nil
						} else {
							// Not valid JSON yet, continue waiting
							if a.debug {
								log.Printf("Attempt %d: Content found but JSON parse failed: %v (length: %d)",
									attempts, err, len(text))
								if len(text) < 200 {
									log.Printf("Content preview: %s", text)
								} else {
									log.Printf("Content preview: %s...", text[:200])
								}
							}
						}
					} else {
						if a.debug {
							log.Printf("Attempt %d: No text content in result", attempts)
						}
					}
				}
			}
		}

		if a.debug {
			log.Printf("JSON not ready yet, waiting %v before retry...", jsonPollInterval)
		}
		select {
		case <-time.After(jsonPollInterval):
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled during JSON polling",
			}, ctx.Err()
		}
	}

	return &Response{
		Success: false,
		Error:   fmt.Sprintf("timeout after %d attempts waiting for valid JSON", attempts),
	}, fmt.Errorf("timeout: failed to parse valid JSON after %d attempts", attempts)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
