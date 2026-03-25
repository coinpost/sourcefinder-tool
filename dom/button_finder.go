package dom

import (
	"fmt"
	"strings"
)

// ButtonFinder provides robust button finding strategies for AI chat sites
type ButtonFinder struct {
	detector *Detector
}

// NewButtonFinder creates a new button finder
func NewButtonFinder(debug bool) *ButtonFinder {
	return &ButtonFinder{
		detector: NewDetector(debug),
	}
}

// ButtonSelectorStrategy represents a button selector strategy with priority
type ButtonSelectorStrategy struct {
	Selector    string
	Type        string // "css", "aria", "text"
	Description string
	Priority    int
}

// GetRecommendedButtonSelectors returns recommended button selectors in priority order
// Based on the methodology provided
func (f *ButtonFinder) GetRecommendedButtonSelectors() []ButtonSelectorStrategy {
	return []ButtonSelectorStrategy{
		{
			Selector:    `button[type="submit"]`,
			Type:        "css",
			Description: "Submit button by type",
			Priority:    1,
		},
		{
			Selector:    `button[aria-label*="send" i]`,
			Type:        "aria",
			Description: "Button with 'send' in aria-label",
			Priority:    2,
		},
		{
			Selector:    `button[aria-label*="submit" i]`,
			Type:        "aria",
			Description: "Button with 'submit' in aria-label",
			Priority:    3,
		},
		{
			Selector:    `button[aria-label="提交"]`,
			Type:        "aria",
			Description: "Button with Chinese 'submit' label (Grok)",
			Priority:    4,
		},
		{
			Selector:    `[data-testid="send-button"]`,
			Type:        "css",
			Description: "Button with test-id",
			Priority:    5,
		},
		{
			Selector:    `button svg`,
			Type:        "css",
			Description: "Button containing SVG icon",
			Priority:    6,
		},
	}
}

// FindSendButtonScript generates JavaScript to find the send button
// This implements Step 6 from the methodology
func (f *ButtonFinder) FindSendButtonScript() string {
	return `() => {
		// Collect all buttons with their metadata
		return [...document.querySelectorAll('button')].map((b, i) => ({
			i,
			text: b.innerText?.trim() || '',
			aria: b.getAttribute('aria-label'),
			type: b.type,
			class: b.className,
			dataTestId: b.getAttribute('data-testid'),
			hasSVG: b.querySelector('svg') !== null,
			visible: b.getBoundingClientRect().width > 0 && b.getBoundingClientRect().height > 0,
			attributes: Array.from(b.attributes).map(attr => attr.name + '=' + attr.value)
		}));
	}`
}

// ClickSendButtonScript generates JavaScript to click the send button
// This implements the most stable sending method from the methodology
func (f *ButtonFinder) ClickSendButtonScript() string {
	return `() => {
		// Try multiple strategies to find and click the send button
		// Grok button: <button type="submit" aria-label="提交" tabindex="0"><svg>...</button>

		// Strategy 1: button[type="submit"] with visible check
		// This matches Grok's submit button
		let btn = document.querySelector('button[type="submit"]');
		if (btn) {
			// Ensure button is visible and not disabled
			const rect = btn.getBoundingClientRect();
			if (rect.width > 0 && rect.height > 0 && !btn.disabled) {
				btn.click();
				return {
					ok: true,
					method: 'type=submit',
					selector: 'button[type="submit"]',
					ariaLabel: btn.getAttribute('aria-label')
				};
			}
		}

		// Strategy 2: Chinese "提交" (Grok specific)
		btn = document.querySelector('button[aria-label="提交"]');
		if (btn) {
			const rect = btn.getBoundingClientRect();
			if (rect.width > 0 && rect.height > 0 && !btn.disabled) {
				btn.click();
				return { ok: true, method: 'aria-label=提交', selector: 'button[aria-label="提交"]' };
			}
		}

		// Strategy 3: aria-label contains "send" (case-insensitive)
		btn = document.querySelector('button[aria-label*="send" i], button[aria-label*="Send" i]');
		if (btn && btn.offsetParent !== null && !btn.disabled) {
			btn.click();
			return { ok: true, method: 'aria-label=send', selector: 'button[aria-label*="send" i]' };
		}

		// Strategy 4: aria-label contains "submit"
		btn = document.querySelector('button[aria-label*="submit" i], button[aria-label*="Submit" i]');
		if (btn && btn.offsetParent !== null && !btn.disabled) {
			btn.click();
			return { ok: true, method: 'aria-label=submit', selector: 'button[aria-label*="submit" i]' };
		}

		// Strategy 5: data-testid
		btn = document.querySelector('[data-testid="send-button"], button[data-testid*="send" i]');
		if (btn && btn.offsetParent !== null) {
			btn.click();
			return { ok: true, method: 'data-testid', selector: '[data-testid="send-button"]' };
		}

		// Strategy 6: Button with SVG icon (common in AI sites)
		const buttonsWithSVG = [...document.querySelectorAll('button')].filter(b =>
			b.querySelector('svg') &&
			b.offsetParent !== null &&
			b.getBoundingClientRect().width > 0 &&
			b.getBoundingClientRect().height > 0
		);

		if (buttonsWithSVG.length > 0) {
			// Prefer buttons without text (icon-only buttons)
			const iconOnlyBtn = buttonsWithSVG.find(b => !b.innerText?.trim());
			if (iconOnlyBtn) {
				iconOnlyBtn.click();
				return { ok: true, method: 'icon_only_svg', selector: 'button with svg (icon-only)' };
			}

			// Otherwise click the first one
			buttonsWithSVG[0].click();
			return { ok: true, method: 'first_svg', selector: 'button with svg (first)' };
		}

		// Strategy 7: Fallback - any visible button with submit-related class
		btn = [...document.querySelectorAll('button')].find(b =>
			b.offsetParent !== null &&
			/btn|submit|send|go/i.test(b.className)
		);

		if (btn) {
			btn.click();
			return { ok: true, method: 'class_match', selector: 'button with submit-related class' };
		}

		return { ok: false, error: 'No send button found' };
	}`
}

// PressEnterScript generates JavaScript to press Enter key
// This is the most stable method for many AI sites
func (f *ButtonFinder) PressEnterScript() string {
	return `() => {
		// Find the focused element or the input element
		let el = document.activeElement;

		// If no focused element, try to find an input
		if (!el || (el.tagName !== 'TEXTAREA' && el.getAttribute('contenteditable') !== 'true')) {
			el = document.querySelector('[contenteditable="true"], textarea, [role="textbox"]');
		}

		if (!el) {
			return { ok: false, error: 'No input element found' };
		}

		// Focus the element
		el.focus();

		// Create and dispatch Enter key event
		const enterEvent = new KeyboardEvent('keydown', {
			key: 'Enter',
			code: 'Enter',
			keyCode: 13,
			which: 13,
			bubbles: true,
			cancelable: true
		});

		el.dispatchEvent(enterEvent);

		return { ok: true, method: 'Enter key', tagName: el.tagName };
	}`
}

// PressShiftEnterScript generates JavaScript to press Shift+Enter
// Used for multi-line input in chat sites
func (f *ButtonFinder) PressShiftEnterScript() string {
	return `() => {
		let el = document.activeElement;

		if (!el || (el.tagName !== 'TEXTAREA' && el.getAttribute('contenteditable') !== 'true')) {
			el = document.querySelector('[contenteditable="true"], textarea, [role="textbox"]');
		}

		if (!el) {
			return { ok: false, error: 'No input element found' };
		}

		el.focus();

		// Create and dispatch Shift+Enter key event
		const shiftEnterEvent = new KeyboardEvent('keydown', {
			key: 'Enter',
			code: 'Enter',
			keyCode: 13,
			which: 13,
			shiftKey: true,
			bubbles: true,
			cancelable: true
		});

		el.dispatchEvent(shiftEnterEvent);

		return { ok: true, method: 'Shift+Enter key', tagName: el.tagName };
	}`
}

// SubmitMethod represents a submission method
type SubmitMethod struct {
	Method   string
	Selector string
	Success  bool
}

// ScanForButtons scans the page for submit buttons
func (f *ButtonFinder) ScanForButtons() string {
	return `() => {
		const buttons = [...document.querySelectorAll('button')];

		return buttons.map((b, i) => {
			const info = {
				i,
				tag: b.tagName,
				text: b.innerText?.trim() || '',
				aria: b.getAttribute('aria-label'),
				type: b.type,
				class: b.className,
				dataTestId: b.getAttribute('data-testid'),
				id: b.id,
				hasSVG: b.querySelector('svg') !== null,
				visible: b.getBoundingClientRect().width > 0 && b.getBoundingClientRect().height > 0,
				disabled: b.disabled
			};

			// Score this button as a potential submit button
			let score = 0;
			let reasons = [];

			if (b.type === 'submit') {
				score += 10;
				reasons.push('type=submit');
			}
			if (b.getAttribute('aria-label')) {
				const aria = b.getAttribute('aria-label').toLowerCase();
				if (aria.includes('send') || aria.includes('submit') || aria === '提交') {
					score += 8;
					reasons.push('aria-label matches');
				}
			}
			if (b.getAttribute('data-testid')?.includes('send')) {
				score += 7;
				reasons.push('data-testid matches');
			}
			if (b.querySelector('svg') && !b.innerText?.trim()) {
				score += 5;
				reasons.push('icon-only button');
			}
			if (b.className && /btn|submit|send|go/i.test(b.className)) {
				score += 3;
				reasons.push('class matches');
			}

			info.score = score;
			info.reasons = reasons;

			return info;
		}).filter(b => b.visible && !b.disabled).sort((a, b) => b.score - a.score);
	}`
}

// ButtonInfo represents information about a button
type ButtonInfo struct {
	Index      int      `json:"index"`
	Tag        string   `json:"tag"`
	Text       string   `json:"text"`
	ARIALabel  string   `json:"aria_label"`
	Type       string   `json:"type"`
	Class      string   `json:"class"`
	DataTestId string   `json:"data_test_id"`
	ID         string   `json:"id"`
	HasSVG     bool     `json:"has_svg"`
	Visible    bool     `json:"visible"`
	Disabled   bool     `json:"disabled"`
	Score      int      `json:"score"`
	Reasons    []string `json:"reasons"`
}

// ParseButtonScanResults parses the result from ScanForButtons
func (f *ButtonFinder) ParseButtonScanResults(result interface{}) ([]ButtonInfo, error) {
	var buttons []ButtonInfo

	if arr, ok := result.([]interface{}); ok {
		for _, item := range arr {
			if btnMap, ok := item.(map[string]interface{}); ok {
				btn := ButtonInfo{}
				if idx, ok := btnMap["i"].(float64); ok {
					btn.Index = int(idx)
				}
				if tag, ok := btnMap["tag"].(string); ok {
					btn.Tag = tag
				}
				if text, ok := btnMap["text"].(string); ok {
					btn.Text = text
				}
				if aria, ok := btnMap["aria"].(string); ok {
					btn.ARIALabel = aria
				}
				if type_, ok := btnMap["type"].(string); ok {
					btn.Type = type_
				}
				if class, ok := btnMap["class"].(string); ok {
					btn.Class = class
				}
				if dataTestId, ok := btnMap["dataTestId"].(string); ok {
					btn.DataTestId = dataTestId
				}
				if id, ok := btnMap["id"].(string); ok {
					btn.ID = id
				}
				if hasSVG, ok := btnMap["hasSVG"].(bool); ok {
					btn.HasSVG = hasSVG
				}
				if visible, ok := btnMap["visible"].(bool); ok {
					btn.Visible = visible
				}
				if disabled, ok := btnMap["disabled"].(bool); ok {
					btn.Disabled = disabled
				}
				if score, ok := btnMap["score"].(float64); ok {
					btn.Score = int(score)
				}
				if reasons, ok := btnMap["reasons"].([]interface{}); ok {
					for _, r := range reasons {
						if reasonStr, ok := r.(string); ok {
							btn.Reasons = append(btn.Reasons, reasonStr)
						}
					}
				}
				buttons = append(buttons, btn)
			}
		}
		return buttons, nil
	}

	return nil, fmt.Errorf("unexpected result type: %T", result)
}

// GetBestButtonSelector returns the best selector based on scan results
func (f *ButtonFinder) GetBestButtonSelector(buttons []ButtonInfo) (string, error) {
	if len(buttons) == 0 {
		return "", fmt.Errorf("no buttons found")
	}

	// Return the highest-scoring button's selector
	bestBtn := buttons[0]

	// Build selector based on available attributes
	if bestBtn.Type == "submit" {
		return fmt.Sprintf("button[type=\"submit\"]:nth-of-type(%d)", bestBtn.Index+1), nil
	}

	if bestBtn.ARIALabel != "" {
		return fmt.Sprintf("button[aria-label=\"%s\"]", bestBtn.ARIALabel), nil
	}

	if bestBtn.DataTestId != "" {
		return fmt.Sprintf("[data-testid=\"%s\"]", bestBtn.DataTestId), nil
	}

	if bestBtn.ID != "" {
		return fmt.Sprintf("#%s", bestBtn.ID), nil
	}

	// Fallback to nth-child
	return fmt.Sprintf("button:nth-of-type(%d)", bestBtn.Index+1), nil
}

// FormatButtonInfo formats a ButtonInfo for logging
func (f *ButtonFinder) FormatButtonInfo(btn ButtonInfo) string {
	parts := []string{fmt.Sprintf("#%d", btn.Index)}
	if btn.Type != "" {
		parts = append(parts, fmt.Sprintf("type=%s", btn.Type))
	}
	if btn.ARIALabel != "" {
		parts = append(parts, fmt.Sprintf("aria=%s", truncateStr(btn.ARIALabel, 20)))
	}
	if btn.Text != "" {
		parts = append(parts, fmt.Sprintf("text=%s", truncateStr(btn.Text, 20)))
	}
	if btn.HasSVG {
		parts = append(parts, "has_svg")
	}
	if btn.Score > 0 {
		parts = append(parts, fmt.Sprintf("score=%d", btn.Score))
	}
	return strings.Join(parts, ", ")
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
