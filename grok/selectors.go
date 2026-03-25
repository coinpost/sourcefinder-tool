package grok

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// Selectors contains CSS selectors and JavaScript for Grok DOM interaction
type Selectors struct{}

// NewSelectors creates a new Selectors instance
func NewSelectors() *Selectors {
	return &Selectors{}
}

// FindQuerybarLocation is a debug helper that prints the location of querybar in the snapshot
func (s *Selectors) FindQuerybarLocation(snapshotText string) string {
	lines := strings.Split(snapshotText, "\n")
	for i, line := range lines {
		if strings.Contains(line, "querybar") {
			start := max(0, i-2)
			end := min(i+3, len(lines))
			context := strings.Join(lines[start:end], "\n")
			return fmt.Sprintf("Found querybar at line %d:\n%s", i, context)
		}
	}
	return "querybar not found in snapshot"
}

// FindInputUID extracts the UID of the Grok input textarea from a snapshot
// The snapshot is a text-formatted tree structure returned by chrome-devtools-mcp
func (s *Selectors) FindInputUID(snapshotText string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(snapshotText))

	var querybarUID string
	var contentEditableUID string
	var candidateUIDs []string

	// First pass: find querybar (including in ignored elements)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Scan for querybar first
	for _, line := range lines {
		// Look for div.querybar (with or without explicit div tag, even in ignored elements)
		if strings.Contains(line, "querybar") {
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "uid=") {
					querybarUID = strings.TrimPrefix(field, "uid=")
					break
				}
			}
		}
	}

	// Second pass: look for contenteditable elements
	// If we found querybar, prefer contenteditable elements that appear after it
	foundQuerybar := querybarUID != ""
	for _, line := range lines {
		// Look for contenteditable elements
		if strings.Contains(line, "contenteditable") || strings.Contains(line, "editable") {
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "uid=") {
					uid := strings.TrimPrefix(field, "uid=")
					// If we found querybar, check if this contenteditable appears in a related context
					if foundQuerybar && (strings.Contains(line, "div") || strings.Contains(line, "span")) {
						contentEditableUID = uid
						return uid, nil // Found contenteditable inside/near querybar
					}
					candidateUIDs = append(candidateUIDs, uid)
				}
			}
		}

		// Look for lines containing "textbox" or "textarea"
		if strings.Contains(line, "textbox") || strings.Contains(line, "textarea") {
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "uid=") {
					uid := strings.TrimPrefix(field, "uid=")
					return uid, nil // Found textarea/textbox, prefer this
				}
			}
		}

		// Look for div elements that might be input areas
		if strings.Contains(line, "div") && (strings.Contains(line, "input") || strings.Contains(line, "prompt") || strings.Contains(line, "message")) {
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "uid=") {
					uid := strings.TrimPrefix(field, "uid=")
					candidateUIDs = append(candidateUIDs, uid)
				}
			}
		}
	}

	// Third pass: if querybar found, look for any generic element near it
	// Grok's input might be marked as "generic" or have other accessibility roles
	if foundQuerybar {
		// Find the querybar line index
		querybarIndex := -1
		for i, line := range lines {
			if strings.Contains(line, "uid="+querybarUID) {
				querybarIndex = i
				break
			}
		}

		// Look in the next 50-100 lines after querybar for input-related elements
		if querybarIndex >= 0 {
			searchLimit := min(querybarIndex+100, len(lines))
			for i := querybarIndex + 1; i < searchLimit; i++ {
				line := lines[i]
				// Look for any element that could be an input
				// Check for common input indicators
				if strings.Contains(line, "uid=") &&
					(strings.Contains(line, "generic") ||
						strings.Contains(line, "textbox") ||
						strings.Contains(line, "textarea") ||
						strings.Contains(line, "edit") ||
						strings.Contains(line, "rich-text") ||
						strings.Contains(line, "article")) {

					// Check if it's not a nested structure item (like StaticText or InlineTextBox)
					if !strings.Contains(line, "StaticText") &&
						!strings.Contains(line, "InlineTextBox") &&
						!strings.Contains(line, "image") {

						fields := strings.Fields(line)
						for _, field := range fields {
							if strings.HasPrefix(field, "uid=") {
								uid := strings.TrimPrefix(field, "uid=")
								return uid, nil
							}
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading snapshot: %w", err)
	}

	// Prefer contenteditable element if found
	if contentEditableUID != "" {
		return contentEditableUID, nil
	}

	// Return first candidate if any
	if len(candidateUIDs) > 0 {
		return candidateUIDs[0], nil
	}

	return "", fmt.Errorf("input element not found in snapshot (tried: querybar, textbox, textarea, contenteditable, div with input/prompt/message, generic elements near querybar)")
}


// FindSendButtonUID extracts the UID of the send button from a snapshot
// The button is inside div.querybar and only appears after text is entered
// Button attributes: class="group flex flex-col justify-center rounded-full focus:outline-none focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring" type="submit" aria-label="提交" tabindex="0"
func (s *Selectors) FindSendButtonUID(snapshotText string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(snapshotText))

	// First pass: find querybar UID
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var querybarUID string
	for _, line := range lines {
		// Look for div.querybar
		if strings.Contains(line, "div") && strings.Contains(line, "querybar") {
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "uid=") {
					querybarUID = strings.TrimPrefix(field, "uid=")
					break
				}
			}
		}
	}

	// If querybar not found, try to find button anyway
	if querybarUID == "" {
		// Fallback: search for button without querybar context
		for _, line := range lines {
			if strings.Contains(line, "button") && strings.Contains(line, "提交") {
				fields := strings.Fields(line)
				for _, field := range fields {
					if strings.HasPrefix(field, "uid=") {
						return strings.TrimPrefix(field, "uid="), nil
					}
				}
			}
		}
		return "", fmt.Errorf("querybar not found in snapshot")
	}

	// Second pass: look for submit button inside/near querybar
	// The submit button has specific characteristics:
	// - type="submit"
	// - aria-label="提交"
	// - tabindex="0"
	// - class contains: "group", "flex", "flex-col", "rounded-full", "focus-visible:ring-ring"
	insideQuerybar := false
	querybarIndentLevel := 0

	for _, line := range lines {
		// Detect when we're inside querybar (indented more than querybar line)
		if strings.Contains(line, "uid="+querybarUID) {
			// Calculate indent level of querybar
			querybarIndentLevel = len(line) - len(strings.TrimLeft(line, " \t"))
			insideQuerybar = true
			continue
		}

		// Exit querybar when we reach a line with less or equal indentation
		if insideQuerybar {
			lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if line != "" && lineIndent <= querybarIndentLevel && !strings.HasPrefix(line, " ") {
				// We've exited the querybar section
				// But don't break yet, the button might still be nearby
			}
		}

		// Look for button with submit type and "提交" aria-label
		if strings.Contains(line, "button") {
			// Check for multiple indicators to confirm it's the submit button
			hasTypeSubmit := strings.Contains(line, "type=\"submit\"") || strings.Contains(line, "type='submit'")
			hasSubmitLabel := strings.Contains(line, "aria-label=\"提交\"") || strings.Contains(line, "aria-label='提交'")
			hasTabIndex := strings.Contains(line, "tabindex=\"0\"") || strings.Contains(line, "tabindex='0'")

			// Look for class attributes as additional confirmation
			hasRoundedClass := strings.Contains(line, "rounded-full")
			hasRingClass := strings.Contains(line, "focus-visible:ring-ring")
			hasGroupClass := strings.Contains(line, "group")

			// If most indicators match, this is likely the submit button
			indicatorCount := 0
			if hasTypeSubmit {
				indicatorCount++
			}
			if hasSubmitLabel {
				indicatorCount++
			}
			if hasTabIndex {
				indicatorCount++
			}
			if hasRoundedClass {
				indicatorCount++
			}
			if hasRingClass {
				indicatorCount++
			}
			if hasGroupClass {
				indicatorCount++
			}

			// Require at least 3 indicators to match
			if indicatorCount >= 3 {
				fields := strings.Fields(line)
				for _, field := range fields {
					if strings.HasPrefix(field, "uid=") {
						uid := strings.TrimPrefix(field, "uid=")
						return uid, nil
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading snapshot: %w", err)
	}

	return "", fmt.Errorf("send button not found in snapshot (searched inside querybar)")
}

// CheckResponseReadyScript returns JavaScript to check if Grok response has started
// This is used for polling - checks for the specific response-content-markdown div
func (s *Selectors) CheckResponseReadyScript() string {
	return `() => {
		// Look for all Grok response containers
		// div class="relative response-content-markdown markdown"
		const responseDivs = document.querySelectorAll('div.response-content-markdown.markdown');

		if (responseDivs.length === 0) {
			// No elements found yet
			return {
				found: false,
				hasContent: false
			};
		}

		// Get the LAST response div (most recent response)
		const responseDiv = responseDivs[responseDivs.length - 1];

		// Element exists, check if it has content
		const text = (responseDiv.textContent || '').trim();

		if (text.length > 10) {
			return {
				found: true,
				hasContent: true,
				length: text.length,
				preview: text.substring(0, 100),
				totalDivs: responseDivs.length
			};
		}

		// Element exists but no content yet (still loading/streaming)
		return {
			found: true,
			hasContent: false,
			streaming: true,
			totalDivs: responseDivs.length
		};
	}`
}

// GetResponseScript returns JavaScript code to extract the latest Grok response
// This extracts JSON code blocks from the LAST response-content-markdown div
func (s *Selectors) GetResponseScript() string {
	return `() => {
		// Find ALL response containers and get the LAST one (most recent)
		const responseDivs = document.querySelectorAll('div.response-content-markdown.markdown');

		if (responseDivs.length === 0) {
			return { error: "response-content-markdown div not found" };
		}

		// Get the LAST response div (most recent response from Grok)
		const responseDiv = responseDivs[responseDivs.length - 1];

		// Try to find JSON code blocks within the response
		// Look for <pre><code> blocks or markdown code blocks
		const codeBlocks = responseDiv.querySelectorAll('pre code, code');

		// Collect all valid JSON blocks
		const jsonBlocks = [];

		for (const codeBlock of codeBlocks) {
			const text = codeBlock.textContent || codeBlock.innerText || '';
			const trimmed = text.trim();

			// Check if this looks like JSON (starts with { or [)
			if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
				try {
					// Try to parse as JSON to verify it's valid
					JSON.parse(trimmed);

					// It's valid JSON, add to collection
					jsonBlocks.push({
						text: trimmed,
						html: codeBlock.innerHTML || '',
						length: trimmed.length
					});
				} catch (e) {
					// Not valid JSON, skip
					continue;
				}
			}
		}

		// If we found JSON blocks, return the LAST one (most complete/recent)
		if (jsonBlocks.length > 0) {
			const lastJson = jsonBlocks[jsonBlocks.length - 1];
			return {
				text: lastJson.text,
				html: lastJson.html,
				timestamp: new Date().toISOString(),
				method: 'json_code_block',
				selector: 'last div.response-content-markdown.markdown code',
				totalDivs: responseDivs.length,
				totalJsonBlocks: jsonBlocks.length
			};
		}

		// If no JSON code block found, try to extract from the entire content
		const fullText = (responseDiv.textContent || responseDiv.innerText || '').trim();

		// Try to find JSON patterns in the text using balanced bracket matching
		// This handles nested objects/arrays correctly
		const findBalancedJson = (text, startChar, endChar) => {
			const results = [];
			let depth = 0;
			let startIndex = -1;

			for (let i = 0; i < text.length; i++) {
				const char = text[i];

				if (char === startChar) {
					if (depth === 0) {
						startIndex = i;
					}
					depth++;
				} else if (char === endChar) {
					depth--;
					if (depth === 0 && startIndex !== -1) {
						// Found a balanced pair
						const candidate = text.substring(startIndex, i + 1);
						try {
							// Verify it's valid JSON
							JSON.parse(candidate);
							results.push({
								text: candidate,
								start: startIndex,
								end: i
							});
						} catch (e) {
							// Not valid JSON, skip
						}
						startIndex = -1;
					}
				}
			}

			return results;
		};

		// Find all balanced JSON objects ({...})
		const objectMatches = findBalancedJson(fullText, '{', '}');
		// Find all balanced JSON arrays ([...])
		const arrayMatches = findBalancedJson(fullText, '[', ']');

		// Combine and sort by position
		const allMatches = [...objectMatches, ...arrayMatches].sort((a, b) => a.start - b.start);

		// If we found JSON patterns, return the LAST one (most complete)
		if (allMatches.length > 0) {
			const lastMatch = allMatches[allMatches.length - 1];
			return {
				text: lastMatch.text,
				html: '',
				timestamp: new Date().toISOString(),
				method: 'balanced_json_match',
				selector: 'last div.response-content-markdown.markdown text',
				totalDivs: responseDivs.length,
				totalJsonMatches: allMatches.length
			};
		}

		// Fallback: return all text content (in case there's no JSON)
		return {
			text: fullText,
			html: responseDiv.innerHTML || '',
			timestamp: new Date().toISOString(),
			method: 'full_text',
			selector: 'last div.response-content-markdown.markdown',
			totalDivs: responseDivs.length
		};
	}`
}

// WaitForPageLoadScript returns JavaScript to check if page is loaded
func (s *Selectors) WaitForPageLoadScript() string {
	return `() => {
		// Check for common Grok page elements
		const inputExists = document.querySelector('textarea') !== null ||
			document.querySelector('input[type="text"]') !== null ||
			document.querySelector('[contenteditable="true"]') !== null;
		const sendButtonExists = document.querySelector('button') !== null;

		return {
			loaded: inputExists || sendButtonExists,
			hasInput: inputExists
		};
	}`
}

// ParseResponseResult parses the result from GetResponseScript
func (s *Selectors) ParseResponseResult(result interface{}) (*GrokMsg, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	if errMsg, exists := resultMap["error"]; exists {
		return nil, fmt.Errorf("JavaScript error: %v", errMsg)
	}

	text, ok := resultMap["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text field missing or not string")
	}

	timestampStr, _ := resultMap["timestamp"].(string)
	timestamp := time.Now()
	if timestampStr != "" {
		if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			timestamp = t
		}
	}

	turnIndex := 0
	if idx, ok := resultMap["turnIndex"].(float64); ok {
		turnIndex = int(idx)
	}

	// HTML field is optional
	html, _ := resultMap["html"].(string)

	return &GrokMsg{
		Text:      text,
		HTML:      html,
		Timestamp: timestamp,
		TurnIndex: turnIndex,
	}, nil
}
