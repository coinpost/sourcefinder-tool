package dom

import (
	"fmt"
	"strings"
)

// InputFinder provides robust input element finding strategies
type InputFinder struct {
	detector *Detector
}

// NewInputFinder creates a new input finder
func NewInputFinder(debug bool) *InputFinder {
	return &InputFinder{
		detector: NewDetector(debug),
	}
}

// SelectorStrategy represents a selector strategy with priority
type SelectorStrategy struct {
	Selector    string
	Type        string // "css", "xpath", "aria"
	Description string
	Priority    int
}

// GetRecommendedSelectors returns recommended selectors in priority order
// Based on the methodology provided
func (f *InputFinder) GetRecommendedSelectors() []SelectorStrategy {
	return []SelectorStrategy{
		{
			Selector:    `[contenteditable="true"]`,
			Type:        "css",
			Description: "Contenteditable div (most common for AI chat sites)",
			Priority:    1,
		},
		{
			Selector:    `textarea`,
			Type:        "css",
			Description: "Textarea element",
			Priority:    2,
		},
		{
			Selector:    `[role="textbox"]`,
			Type:        "css",
			Description: "ARIA textbox role",
			Priority:    3,
		},
		{
			Selector:    `textarea[placeholder]`,
			Type:        "css",
			Description: "Textarea with placeholder",
			Priority:    4,
		},
		{
			Selector:    `[aria-label*="message" i]`,
			Type:        "css",
			Description: "Element with 'message' in aria-label",
			Priority:    5,
		},
		{
			Selector:    `[placeholder*="message" i]`,
			Type:        "css",
			Description: "Element with 'message' in placeholder",
			Priority:    6,
		},
		{
			Selector:    `input[type="text"]`,
			Type:        "css",
			Description: "Text input field",
			Priority:    7,
		},
	}
}

// FindInputScript generates JavaScript to find and interact with an input element
// This implements Step 5 from the methodology
func (f *InputFinder) FindInputScript(text string) string {
	// Check if text contains HTML tags
	containsHTML := strings.Contains(text, "<") && strings.Contains(text, ">")

	// Use %q which automatically escapes the string for JavaScript
	return fmt.Sprintf(`() => {
		const textContent = %q;
		const containsHTML = %t;

		// Try selectors in priority order
		// Note: contenteditable elements support HTML, while textarea/input only support plain text
		const selectors = [
			'[contenteditable="true"]',
			'[role="textbox"][contenteditable]',
			'div[contenteditable]',
			'span[contenteditable]',
			'[role="textbox"]',
			'textarea',
			'textarea[placeholder]',
			'[aria-label*="message" i]',
			'[placeholder*="message" i]',
			'input[type="text"]'
		];

		for (const selector of selectors) {
			const el = document.querySelector(selector);
			if (el) {
				// Check if element is visible
				const rect = el.getBoundingClientRect();
				if (rect.width > 0 && rect.height > 0) {
					// Focus the element
					el.focus();

					// Set the value based on element type
					const isContentEditable = el.getAttribute('contenteditable') === 'true' ||
					                          el.getAttribute('contenteditable') === '' ||
					                          el.contentEditable === 'true';

					if (isContentEditable) {
						// For contenteditable divs
						if (containsHTML) {
							// Use innerHTML for HTML content
							el.innerHTML = textContent;
							el.dispatchEvent(new InputEvent('input', { bubbles: true }));
							el.dispatchEvent(new Event('change', { bubbles: true }));
							return {
								ok: true,
								selector: selector,
								tag: el.tagName,
								method: 'innerHTML',
								contentEditableAttr: el.getAttribute('contenteditable'),
								containsHTML: containsHTML,
								textLength: textContent.length
							};
						} else {
							// Use textContent for plain text
							el.textContent = textContent;
							el.dispatchEvent(new InputEvent('input', { bubbles: true }));
							el.dispatchEvent(new Event('change', { bubbles: true }));
							return {
								ok: true,
								selector: selector,
								tag: el.tagName,
								method: 'textContent',
								contentEditableAttr: el.getAttribute('contenteditable'),
								containsHTML: containsHTML,
								textLength: textContent.length
							};
						}
					} else if (el.value !== undefined) {
						// For textarea/input - always use value (plain text only)
						el.value = textContent;
						el.dispatchEvent(new Event('input', { bubbles: true }));
						el.dispatchEvent(new Event('change', { bubbles: true }));
						return {
							ok: true,
							selector: selector,
							tag: el.tagName,
							method: 'value',
							warning: 'textarea/input only supports plain text, HTML tags will be displayed as text',
							containsHTML: containsHTML,
							textLength: textContent.length
						};
					}

					return {
						ok: true,
						selector: selector,
						tag: el.tagName,
						method: el.getAttribute('contenteditable') === 'true' ? 'contenteditable' : 'value'
					};
				}
			}
		}

		return { ok: false, error: 'No suitable input element found' };
	}`, text, containsHTML)
}

// FocusAndTypeScript generates JavaScript to focus and type into an element
func (f *InputFinder) FocusAndTypeScript(selector string, text string) string {
	return fmt.Sprintf(`async () => {
		const el = document.querySelector('%s');
		if (!el) return { ok: false, error: 'Element not found' };

		// Check visibility
		const rect = el.getBoundingClientRect();
		if (rect.width === 0 || rect.height === 0) {
			return { ok: false, error: 'Element is not visible' };
		}

		// Focus the element
		el.focus();

		// Wait a bit for focus to take effect
		await new Promise(resolve => setTimeout(resolve, 100));

		// Set the value
		if (el.getAttribute('contenteditable') === 'true') {
			el.textContent = %q;
			el.dispatchEvent(new InputEvent('input', { bubbles: true }));
		} else if (el.value !== undefined) {
			el.value = %q;
			el.dispatchEvent(new Event('input', { bubbles: true }));
		}

		return { ok: true, selector: '%s' };
	}`, selector, text, text, selector)
}

// ClearInputScript generates JavaScript to clear an input field
func (f *InputFinder) ClearInputScript(selector string) string {
	return fmt.Sprintf(`() => {
		const el = document.querySelector('%s');
		if (!el) return { ok: false, error: 'Element not found' };

		el.focus();

		if (el.getAttribute('contenteditable') === 'true') {
			el.textContent = '';
			el.dispatchEvent(new Event('input', { bubbles: true }));
		} else if (el.value !== undefined) {
			el.value = '';
			el.dispatchEvent(new Event('input', { bubbles: true }));
		}

		return { ok: true };
	}`, selector)
}

// ScanForInputElements scans the page for input elements using all available methods
// This is a comprehensive scan that checks main DOM, iframes, and Shadow DOM
func (f *InputFinder) ScanForInputElements() string {
	return `() => {
		const result = {
			mainDOM: [],
			iframes: [],
			shadowDOM: []
		};

		// Scan main DOM
		result.mainDOM = [...document.querySelectorAll(
			'textarea, input, [contenteditable="true"], [role="textbox"]'
		)].map((el, i) => ({
			i,
			tag: el.tagName,
			id: el.id,
			class: el.className,
			role: el.getAttribute('role'),
			contenteditable: el.getAttribute('contenteditable'),
			placeholder: el.getAttribute('placeholder'),
			aria: el.getAttribute('aria-label'),
			visible: el.getBoundingClientRect().width > 0 && el.getBoundingClientRect().height > 0
		}));

		// Scan iframes
		result.iframes = [...document.querySelectorAll('iframe')].map((f, i) => ({
			i,
			src: f.src,
			id: f.id,
			class: f.className,
			sameOrigin: f.src === '' || f.src.startsWith(window.location.origin)
		}));

		// Scan Shadow DOM hosts
		result.shadowDOM = [...document.querySelectorAll('*')]
			.filter(el => el.shadowRoot)
			.map((el, i) => {
				const input = el.shadowRoot.querySelector(
					'textarea, input, [contenteditable="true"], [role="textbox"]'
				);
				return {
					i,
					hostTag: el.tagName,
					hostId: el.id,
					hostClass: el.className,
					hasInput: input !== null,
					inputTag: input ? input.tagName : null,
					inputId: input ? input.id : null
				};
			});

		return result;
	}`
}

// AnalyzeScanResults analyzes the scan results and recommends the best approach
func (f *InputFinder) AnalyzeScanResults(result interface{}) (*ScanAnalysis, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	analysis := &ScanAnalysis{
		Context:      "main",
		SelectorType: "css",
	}

	// Check main DOM
	if mainDOM, ok := resultMap["mainDOM"].([]interface{}); ok {
		if len(mainDOM) > 0 {
			analysis.AvailableInMain = true
			analysis.MainDOMElementCount = len(mainDOM)

			// Find first visible element
			for _, elem := range mainDOM {
				if elemMap, ok := elem.(map[string]interface{}); ok {
					if visible, ok := elemMap["visible"].(bool); ok && visible {
						if tag, ok := elemMap["tag"].(string); ok {
							if tag == "DIV" || tag == "div" {
								analysis.RecommendedSelector = "[contenteditable=\"true\"]"
								analysis.InputType = "contenteditable"
							} else if tag == "TEXTAREA" || tag == "textarea" {
								analysis.RecommendedSelector = "textarea"
								analysis.InputType = "textarea"
							} else {
								analysis.RecommendedSelector = "[role=\"textbox\"]"
								analysis.InputType = "role_textbox"
							}
							break
						}
					}
				}
			}
		}
	}

	// Check iframes
	if iframes, ok := resultMap["iframes"].([]interface{}); ok {
		if len(iframes) > 0 {
			analysis.HasIframes = true
			analysis.IframeCount = len(iframes)

			// Check for same-origin iframes
			for _, iframe := range iframes {
				if iframeMap, ok := iframe.(map[string]interface{}); ok {
					if sameOrigin, ok := iframeMap["sameOrigin"].(bool); ok && sameOrigin {
						analysis.HasSameOriginIframes = true
						break
					}
				}
			}

			// If no elements in main DOM but we have iframes, recommend iframe context
			if !analysis.AvailableInMain {
				analysis.Context = "iframe"
			}
		}
	}

	// Check Shadow DOM
	if shadowDOM, ok := resultMap["shadowDOM"].([]interface{}); ok {
		if len(shadowDOM) > 0 {
			analysis.HasShadowDOM = true
			analysis.ShadowHostCount = len(shadowDOM)

			// Check for inputs in Shadow DOM
			for _, host := range shadowDOM {
				if hostMap, ok := host.(map[string]interface{}); ok {
					if hasInput, ok := hostMap["hasInput"].(bool); ok && hasInput {
						analysis.HasInputInShadowDOM = true
						break
					}
				}
			}

			// If no elements in main DOM or iframes, but we have Shadow DOM inputs
			if !analysis.AvailableInMain && analysis.HasInputInShadowDOM {
				analysis.Context = "shadow_dom"
			}
		}
	}

	return analysis, nil
}

// ScanAnalysis represents the analysis of a scan
type ScanAnalysis struct {
	Context                 string `json:"context"`        // "main", "iframe", "shadow_dom"
	SelectorType            string `json:"selector_type"`  // "css", "xpath", "aria"
	AvailableInMain         bool   `json:"available_in_main"`
	MainDOMElementCount     int    `json:"main_dom_element_count"`
	RecommendedSelector     string `json:"recommended_selector"`
	InputType               string `json:"input_type"`     // "contenteditable", "textarea", "input"
	HasIframes              bool   `json:"has_iframes"`
	IframeCount             int    `json:"iframe_count"`
	HasSameOriginIframes    bool   `json:"has_same_origin_iframes"`
	HasShadowDOM            bool   `json:"has_shadow_dom"`
	ShadowHostCount         int    `json:"shadow_host_count"`
	HasInputInShadowDOM     bool   `json:"has_input_in_shadow_dom"`
}

// GetRecommendedAction returns a human-readable recommended action
func (a *ScanAnalysis) GetRecommendedAction() string {
	if a.AvailableInMain {
		return fmt.Sprintf("Use selector '%s' in main DOM context (input type: %s, found %d elements)",
			a.RecommendedSelector, a.InputType, a.MainDOMElementCount)
	}

	if a.HasInputInShadowDOM {
		return fmt.Sprintf("Input found in Shadow DOM (found %d Shadow hosts with inputs) - use Shadow DOM access",
			a.ShadowHostCount)
	}

	if a.HasIframes {
		if a.HasSameOriginIframes {
			return fmt.Sprintf("Input likely in iframe (found %d iframes, %d same-origin) - switch to iframe context",
				a.IframeCount, a.IframeCount)
		}
		return fmt.Sprintf("Input may be in cross-origin iframe (found %d iframes) - cannot access due to CORS",
			a.IframeCount)
	}

	return "No input elements found - page may still be loading or structure is unknown"
}
