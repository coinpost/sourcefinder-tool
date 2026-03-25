package chatgpt

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// Selectors contains CSS selectors and JavaScript for ChatGPT DOM interaction
type Selectors struct{}

// NewSelectors creates a new Selectors instance
func NewSelectors() *Selectors {
	return &Selectors{}
}

// FillInputWithHTMLScript generates JavaScript to fill #prompt-textarea with HTML using innerHTML
func (s *Selectors) FillInputWithHTMLScript(htmlContent string) string {
	return fmt.Sprintf(`() => {
		const textarea = document.querySelector('#prompt-textarea');
		if (!textarea) {
			return { ok: false, error: '#prompt-textarea not found' };
		}

		// Focus the textarea
		textarea.focus();

		// Set HTML content using innerHTML
		textarea.innerHTML = %q;

		// Dispatch events to trigger React's change detection
		textarea.dispatchEvent(new InputEvent('input', { bubbles: true }));
		textarea.dispatchEvent(new Event('change', { bubbles: true }));

		return {
			ok: true,
			selector: '#prompt-textarea',
			method: 'innerHTML',
			length: %q.length
		};
	}`, htmlContent, htmlContent)
}

// FindInputUID extracts the UID of the ChatGPT input textarea from a snapshot
// The snapshot is a text-formatted tree structure returned by chrome-devtools-mcp
func (s *Selectors) FindInputUID(snapshotText string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(snapshotText))

	for scanner.Scan() {
		line := scanner.Text()

		// Look for lines containing "textbox" - that's the input field
		if strings.Contains(line, "textbox") {
			// Extract the UID from the line
			// Line format: "uid=1_156 textbox focusable focused multiline"
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasPrefix(field, "uid=") {
					uid := strings.TrimPrefix(field, "uid=")
					return uid, nil
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading snapshot: %w", err)
	}

	return "", fmt.Errorf("textbox not found in snapshot")
}

// FindSendButtonUID extracts the UID of the send button from a snapshot
func (s *Selectors) FindSendButtonUID(snapshotText string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(snapshotText))

	for scanner.Scan() {
		line := scanner.Text()

		// Look for lines containing "button" and send button text
		// Handle both English and Chinese: "Send message", "发送提示", "发送"
		if strings.Contains(line, "button") {
			lowerLine := strings.ToLower(line)
			// Check for send button indicators in both languages
			if strings.Contains(lowerLine, "send") ||
			   strings.Contains(line, "发送") ||
			   strings.Contains(line, "[data-testid=\"send-button\"]") ||
			   strings.Contains(line, "aria-label=\"Send message\"") {

				// Extract the UID from the line
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

	return "", fmt.Errorf("send button not found in snapshot")
}

// GetResponseScript returns JavaScript code to extract the latest ChatGPT response
func (s *Selectors) GetResponseScript() string {
	return `() => {
  // Find ALL div.cm-content elements and get the LAST one (most recent response)
  const allDivs = document.querySelectorAll('div.cm-content');

  if (allDivs.length === 0) {
    return { error: "div.cm-content not found" };
  }

  // Get the LAST div.cm-content element (most recent response)
  const readonlyDiv = allDivs[allDivs.length - 1];

  // Extract the text content
  const text = readonlyDiv.textContent || readonlyDiv.innerText || '';

  return {
    text: text,
    timestamp: new Date().toISOString(),
    method: 'cm_content_last',
    totalDivs: allDivs.length
  };
}`
}

// DiagnosePageScript returns JavaScript to diagnose page structure
func (s *Selectors) DiagnosePageScript() string {
	return `() => {
  const result = {
    url: window.location.href,
    cmContentDivs: [],
    allDivs: 0,
    textareas: 0,
    buttons: 0
  };

  // Find all div.cm-content elements
  const cmDivs = document.querySelectorAll('div.cm-content');
  result.cmContentDivs = Array.from(cmDivs).map((div, i) => ({
    index: i,
    classes: div.className,
    textLength: (div.textContent || '').length,
    textPreview: (div.textContent || '').substring(0, 100),
    visible: div.offsetParent !== null
  }));

  // Count other elements
  result.allDivs = document.querySelectorAll('div').length;
  result.textareas = document.querySelectorAll('textarea').length;
  result.buttons = document.querySelectorAll('button').length;

  return result;
}`
}

// WaitForPageLoadScript returns JavaScript to check if page is loaded
func (s *Selectors) WaitForPageLoadScript() string {
	return `() => {
  // Check for common ChatGPT page elements
  const inputExists = document.querySelector('#prompt-textarea') !== null;
  const sendButtonExists = document.querySelector('[data-testid="send-button"]') !== null;
  const messageInputExists = document.querySelector('textarea[data-id="request-:"]') !== null;

  return {
    loaded: inputExists || sendButtonExists || messageInputExists,
    hasInput: inputExists || messageInputExists
  };
}`
}

// ParseResponseResult parses the result from GetResponseScript
func (s *Selectors) ParseResponseResult(result interface{}) (*ChatGPTMsg, error) {
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

	return &ChatGPTMsg{
		Text:      text,
		HTML:      html,
		Timestamp: timestamp,
		TurnIndex: turnIndex,
	}, nil
}
