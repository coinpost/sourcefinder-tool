package chatgpt

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

// Agent handles ChatGPT automation using chrome-devtools-mcp
type Agent struct {
	cdt       *mcp.ChromeDevTools
	selectors *Selectors
	timeout   time.Duration
	debug     bool
}

// NewAgent creates a new ChatGPT automation agent
func NewAgent(cdt *mcp.ChromeDevTools, timeout time.Duration, debug bool) *Agent {
	return &Agent{
		cdt:       cdt,
		selectors: NewSelectors(),
		timeout:   timeout,
		debug:     debug,
	}
}

// ProcessDocument sends a document to ChatGPT and returns the response
func (a *Agent) ProcessDocument(ctx context.Context, documentContent string) (*Response, error) {
	startTime := time.Now()

	// Step 1: Navigate to ChatGPT
	if a.debug {
		log.Printf("Navigating to https://chatgpt.com/")
	}
	if err := a.cdt.NavigatePage(ctx, "https://chatgpt.com/"); err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to navigate: %v", err),
		}, fmt.Errorf("navigation failed: %w", err)
	}

	// Step 2: Wait for page to load completely
	if a.debug {
		log.Printf("Waiting for page to load...")
	}
	if err := a.cdt.WaitFor(ctx, []string{"Message ChatGPT", "Send message"}, 10000); err != nil {
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

	// Step 10: Poll for response content in div.cm-content
	if a.debug {
		log.Printf("Polling for div.cm-content to have content...")
	}

	pollInterval := 3 * time.Second
	deadline := time.Now().Add(a.timeout)
	contentFound := false

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

		// Check if response has content (multi-strategy approach)
		checkScript := `() => {
			// Strategy 1: Check old div.cm-content structure
			const cmContentDivs = document.querySelectorAll('div.cm-content');
			if (cmContentDivs.length > 0) {
				const div = cmContentDivs[cmContentDivs.length - 1];
				const text = div.textContent || div.innerText || '';
				if (text.trim().length > 0) {
					return { found: true, hasContent: true, length: text.trim().length, method: 'cm_content' };
				}
			}

			// Strategy 2: Check new section[data-message-author-role="assistant"] structure
			const assistantSections = document.querySelectorAll('section[data-message-author-role="assistant"]');
			if (assistantSections.length > 0) {
				const section = assistantSections[assistantSections.length - 1];
				const markdownDiv = section.querySelector('div.markdown.prose');
				const targetDiv = markdownDiv || section;
				const text = targetDiv.textContent || targetDiv.innerText || '';
				if (text.trim().length > 0) {
					return { found: true, hasContent: true, length: text.trim().length, method: 'assistant_section' };
				}
			}

			return { found: false, hasContent: false, method: 'none' };
		}`

		result, err := a.cdt.EvaluateScript(ctx, checkScript)
		if err != nil {
			if a.debug {
				log.Printf("Warning: check script failed: %v", err)
			}
		} else {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if found, ok := resultMap["found"].(bool); ok && found {
					if hasContent, ok := resultMap["hasContent"].(bool); ok && hasContent {
						methodStr := ""
						if method, ok := resultMap["method"].(string); ok {
							methodStr = method
						}
						if a.debug {
							log.Printf("Response found via %s! (%d chars)", methodStr, int(resultMap["length"].(float64)))
						}
						contentFound = true
						break
					}
				}
			}
		}

		// Wait before next poll
		if a.debug {
			log.Printf("No content yet, waiting %v...", pollInterval)
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

	if !contentFound {
		// Run diagnostics to help debug
		if a.debug {
			log.Printf("⚠️  No content found, running diagnostics...")
			diagScript := a.selectors.DiagnosePageScript()
			diagResult, err := a.cdt.EvaluateScript(ctx, diagScript)
			if err != nil {
				log.Printf("⚠️  Diagnostics failed: %v", err)
			} else {
				diagJSON, _ := json.MarshalIndent(diagResult, "", "  ")
				log.Printf("⚠️  Page diagnostics:\n%s", string(diagJSON))
			}
		}

		return &Response{
			Success: false,
			Error:   "timeout waiting for content in div.cm-content",
		}, fmt.Errorf("timeout: no content found")
	}

	// Step 11: Wait additional 10 seconds for response to complete
	if a.debug {
		log.Printf("Content found, waiting 10 seconds for response to complete...")
	}
	time.Sleep(10 * time.Second)

	// Step 12: Extract the response
	if a.debug {
		log.Printf("Extracting response from div.cm-content...")
	}
	responseMsg, err := a.scrapeResponse(ctx)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to scrape response: %v", err),
		}, fmt.Errorf("scrape response failed: %w", err)
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
}

// scrapeResponse extracts the ChatGPT response from the page
func (a *Agent) scrapeResponse(ctx context.Context) (*ChatGPTMsg, error) {
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

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// NavigateAndSubmit navigates to ChatGPT and submits input using JavaScript (can be parallel)
// This is Phase 2 of the three-phase processing: navigation → input → submit
func (a *Agent) NavigateAndSubmit(ctx context.Context, documentContent string) error {
	// Step 1: Navigate to ChatGPT
	if a.debug {
		log.Printf("Navigating to https://chatgpt.com/")
	}
	if err := a.cdt.NavigatePage(ctx, "https://chatgpt.com/"); err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}

	// Step 2: Wait for page to load completely
	if a.debug {
		log.Printf("Waiting for page to load...")
	}
	if err := a.cdt.WaitFor(ctx, []string{"Message ChatGPT", "Send message"}, 10000); err != nil {
		if a.debug {
			log.Printf("Wait failed, trying alternative approach...")
		}
		time.Sleep(2 * time.Second)
	}

	// Additional wait to ensure page is fully ready
	if a.debug {
		log.Printf("Waiting additional 2 seconds for page to stabilize...")
	}
	time.Sleep(2 * time.Second)

	// Step 3: Use JavaScript to set input (works in parallel)
	if a.debug {
		log.Printf("Setting input content via JavaScript (%d chars)...", len(documentContent))
	}

	// Escape the document content for JavaScript
	escapedContent := strings.ReplaceAll(documentContent, "\\", "\\\\")
	escapedContent = strings.ReplaceAll(escapedContent, "\"", "\\\"")
	escapedContent = strings.ReplaceAll(escapedContent, "\n", "\\n")
	escapedContent = strings.ReplaceAll(escapedContent, "\r", "\\r")
	escapedContent = strings.ReplaceAll(escapedContent, "\t", "\\t")

	setInputScript := fmt.Sprintf(`() => {
		// Use jQuery-style selector (querySelector)
		const textarea = document.querySelector('#prompt-textarea');
		if (!textarea) {
			return { error: "textarea #prompt-textarea not found" };
		}

		// Set the value directly
		textarea.value = %q;

		// Trigger multiple events to ensure ChatGPT recognizes the change
		textarea.dispatchEvent(new Event('input', { bubbles: true }));
		textarea.dispatchEvent(new Event('change', { bubbles: true }));

		// Also use the native setter for React-based components
		const nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
		nativeInputValueSetter.call(textarea, %q);

		return {
			success: true,
			length: %d,
			preview: textarea.value.substring(0, 100)
		};
	}`, escapedContent, escapedContent, len(documentContent))

	result, err := a.cdt.EvaluateScript(ctx, setInputScript)
	if err != nil {
		return fmt.Errorf("failed to set input: %w", err)
	}

	if a.debug {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		log.Printf("Input set result: %s", string(resultJSON))
	}

	// Small wait to ensure input is registered
	time.Sleep(500 * time.Millisecond)

	// Step 4: Wait before sending (1-3 seconds random delay)
	randomDelay := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
	if a.debug {
		log.Printf("Waiting %v before sending...", randomDelay)
	}
	time.Sleep(randomDelay)

	// Step 5: Submit by clicking send button via JavaScript (fully parallel)
	if a.debug {
		log.Printf("Submitting via JavaScript click...")
	}

	submitScript := `() => {
		// Try multiple selectors for the send button
		const selectors = [
			'[data-testid="send-button"]',
			'button[aria-label="Send message"]',
			'button:has(svg)',
			'button[type="submit"]'
		];

		for (const selector of selectors) {
			const button = document.querySelector(selector);
			if (button) {
				button.click();
				return { success: true, selector: selector };
			}
		}

		return { error: "Send button not found" };
	}`

	result, err = a.cdt.EvaluateScript(ctx, submitScript)
	if err != nil {
		return fmt.Errorf("failed to click send button: %w", err)
	}

	if a.debug {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		log.Printf("Submit result: %s", string(resultJSON))
	}

	if a.debug {
		log.Printf("Input submitted successfully")
	}

	return nil
}

// WaitForResponse waits for ChatGPT response and extracts result (can be parallel)
// This is Phase 3 of the three-phase processing: wait → extract
func (a *Agent) WaitForResponse(ctx context.Context) (*Response, error) {
	if a.debug {
		log.Printf("Waiting for ChatGPT response...")
	}

	pollInterval := 3 * time.Second
	deadline := time.Now().Add(a.timeout)
	contentFound := false

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return &Response{
				Success: false,
				Error:   "context cancelled",
			}, ctx.Err()
		default:
		}

		// Check if div.cm-content has content
		checkScript := `() => {
			const allDivs = document.querySelectorAll('div.cm-content');
			if (allDivs.length === 0) {
				return { found: false, hasContent: false };
			}
			// Get the LAST div.cm-content (most recent response)
			const div = allDivs[allDivs.length - 1];
			const text = div.textContent || div.innerText || '';
			return {
				found: true,
				hasContent: text.trim().length > 0,
				length: text.trim().length,
				totalDivs: allDivs.length
			};
		}`

		result, err := a.cdt.EvaluateScript(ctx, checkScript)
		if err != nil {
			if a.debug {
				log.Printf("Warning: check script failed: %v", err)
			}
		} else {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if found, ok := resultMap["found"].(bool); ok && found {
					if hasContent, ok := resultMap["hasContent"].(bool); ok && hasContent {
						if a.debug {
							log.Printf("div.cm-content has content! (%d chars)", int(resultMap["length"].(float64)))
						}
						contentFound = true
						break
					}
				}
			}
		}

		if a.debug {
			log.Printf("No content yet, waiting %v...", pollInterval)
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

	if !contentFound {
		// Run diagnostics to help debug
		if a.debug {
			log.Printf("⚠️  No content found, running diagnostics...")
			diagScript := a.selectors.DiagnosePageScript()
			diagResult, err := a.cdt.EvaluateScript(ctx, diagScript)
			if err != nil {
				log.Printf("⚠️  Diagnostics failed: %v", err)
			} else {
				diagJSON, _ := json.MarshalIndent(diagResult, "", "  ")
				log.Printf("⚠️  Page diagnostics:\n%s", string(diagJSON))
			}
		}

		return &Response{
			Success: false,
			Error:   "timeout waiting for content in div.cm-content",
		}, fmt.Errorf("timeout: no content found")
	}

	// Wait additional 10 seconds for response to complete
	if a.debug {
		log.Printf("Content found, waiting 10 seconds for response to complete...")
	}
	time.Sleep(10 * time.Second)

	// Extract the response
	if a.debug {
		log.Printf("Extracting response from div.cm-content...")
	}
	responseMsg, err := a.scrapeResponse(ctx)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to scrape response: %v", err),
		}, fmt.Errorf("scrape response failed: %w", err)
	}

	return &Response{
		Success:  true,
		Response: *responseMsg,
	}, nil
}

// NavigateAndSubmitWithNewTab opens a new tab via chrome-devtools-mcp and processes the input
func (a *Agent) NavigateAndSubmitWithNewTab(ctx context.Context, documentContent string) error {
	// Step 1: Open new page with ChatGPT URL
	if a.debug {
		log.Printf("Opening new tab with https://chatgpt.com/...")
	}

	if _, err := a.cdt.NewPage(ctx, "https://chatgpt.com/"); err != nil {
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

// InputAndSubmitOnly only does input and submit, assuming already on ChatGPT page
func (a *Agent) InputAndSubmitOnly(ctx context.Context, documentContent string) error {
	// Wait for page to stabilize
	if a.debug {
		log.Printf("Waiting additional 2 seconds for page to stabilize...")
	}
	time.Sleep(2 * time.Second)

	// Use JavaScript to directly fill #prompt-textarea with innerHTML
	if a.debug {
		log.Printf("Setting input content using innerHTML (%d chars)...", len(documentContent))
	}

	fillScript := a.selectors.FillInputWithHTMLScript(documentContent)
	result, err := a.cdt.EvaluateScript(ctx, fillScript)
	if err != nil {
		return fmt.Errorf("failed to execute fill script: %w", err)
	}

	// Check result
	if resultMap, ok := result.(map[string]interface{}); ok {
		if ok, exists := resultMap["ok"].(bool); exists && ok {
			if a.debug {
				log.Printf("✓ Successfully filled input using innerHTML")
				log.Printf("  - Selector: %v", resultMap["selector"])
				log.Printf("  - Method: %v", resultMap["method"])
				log.Printf("  - Length: %v", resultMap["length"])
			}
		} else if errMsg, exists := resultMap["error"].(string); exists {
			return fmt.Errorf("fill input failed: %s", errMsg)
		}
	} else {
		return fmt.Errorf("unexpected fill result: %v", result)
	}

	// Wait a moment to ensure content is set
	time.Sleep(500 * time.Millisecond)

	// Wait before sending (1-3 seconds random delay)
	randomDelay := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
	if a.debug {
		log.Printf("Waiting %v before sending...", randomDelay)
	}
	time.Sleep(randomDelay)

	// Take snapshot to find send button
	if a.debug {
		log.Printf("Taking snapshot to find send button...")
	}
	snapshot, err := a.cdt.TakeSnapshot(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to take snapshot: %w", err)
	}

	if a.debug {
		log.Printf("Snapshot (first 5000 chars):\n%s", truncateString(snapshot, 5000))
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
