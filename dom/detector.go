package dom

import (
	"fmt"
	"strings"
)

// Detector provides generic DOM detection capabilities for AI chat sites
type Detector struct {
	debug bool
}

// NewDetector creates a new DOM detector
func NewDetector(debug bool) *Detector {
	return &Detector{
		debug: debug,
	}
}

// ScanResult represents the result of a DOM scan
type ScanResult struct {
	Found       bool     `json:"found"`
	Context     string   `json:"context,omitempty"`
	Elements    []ElementInfo `json:"elements,omitempty"`
	Iframes     []IframeInfo  `json:"iframes,omitempty"`
	ShadowHosts []ShadowHostInfo `json:"shadow_hosts,omitempty"`
}

// ElementInfo represents information about a DOM element
type ElementInfo struct {
	Index       int    `json:"index"`
	Tag         string `json:"tag"`
	ID          string `json:"id,omitempty"`
	Class       string `json:"class,omitempty"`
	Role        string `json:"role,omitempty"`
	ContentEditable string `json:"contenteditable,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	ARIALabel   string `json:"aria_label,omitempty"`
}

// IframeInfo represents information about an iframe
type IframeInfo struct {
	Index int    `json:"index"`
	Src   string `json:"src,omitempty"`
	ID    string `json:"id,omitempty"`
	Class string `json:"class,omitempty"`
}

// ShadowHostInfo represents information about a Shadow DOM host
type ShadowHostInfo struct {
	Index         int    `json:"index"`
	Tag           string `json:"tag"`
	ID            string `json:"id,omitempty"`
	Class         string `json:"class,omitempty"`
	ShadowChildren int   `json:"shadow_children"`
}

// Step 1: Scan for input elements in current context
func (d *Detector) ScanInputElements() string {
	return `(() => {
		return [...document.querySelectorAll(
			'textarea, input, [contenteditable="true"], [role="textbox"]'
		)].map((el, i) => ({
			i,
			tag: el.tagName,
			id: el.id,
			class: el.className,
			role: el.getAttribute('role'),
			contenteditable: el.getAttribute('contenteditable'),
			placeholder: el.getAttribute('placeholder'),
			aria: el.getAttribute('aria-label')
		}));
	})()`
}

// Step 2: Scan for iframes
func (d *Detector) ScanIframes() string {
	return `(() => {
		return [...document.querySelectorAll('iframe')].map((f, i) => ({
			i,
			src: f.src,
			id: f.id,
			class: f.className
		}));
	})()`
}

// Step 3: Scan for Shadow DOM hosts
func (d *Detector) ScanShadowHosts() string {
	return `(() => {
		return [...document.querySelectorAll('*')]
			.filter(el => el.shadowRoot)
			.map((el, i) => ({
				i,
				tag: el.tagName,
				id: el.id,
				class: el.className,
				shadowChildren: el.shadowRoot.children.length
			}));
	})()`
}

// Step 3 (advanced): Scan for input elements inside Shadow DOM
func (d *Detector) ScanShadowDOMInputs() string {
	return `(() => {
		const result = [];
		for (const host of document.querySelectorAll('*')) {
			if (!host.shadowRoot) continue;

			const found = host.shadowRoot.querySelector(
				'textarea, input, [contenteditable="true"], [role="textbox"]'
			);

			if (found) {
				result.push({
					host: host.tagName,
					hostId: host.id,
					hostClass: host.className,
					foundTag: found.tagName,
					foundId: found.id,
					foundClass: found.className,
					placeholder: found.getAttribute('placeholder'),
					aria: found.getAttribute('aria-label')
				});
			}
		}
		return result;
	})()`
}

// DetectContext analyzes the page to determine where input elements are located
// Returns: "main", "iframe", or "shadow_dom"
func (d *Detector) DetectContext(mainContextResult interface{}) string {
	// Check if we got an array of elements (direct result from querySelectorAll)
	if elements, ok := mainContextResult.([]interface{}); ok {
		if len(elements) > 0 {
			return "main"
		}
	}

	// If empty array or not found
	if elements, ok := mainContextResult.([]interface{}); ok && len(elements) == 0 {
		return "unknown"
	}

	return "unknown"
}

// ParseScanResults parses the result from ScanInputElements
func (d *Detector) ParseScanResults(result interface{}) ([]ElementInfo, error) {
	var elements []ElementInfo

	// Handle array result
	if arr, ok := result.([]interface{}); ok {
		for _, item := range arr {
			if elemMap, ok := item.(map[string]interface{}); ok {
				elem := ElementInfo{}
				if idx, ok := elemMap["i"].(float64); ok {
					elem.Index = int(idx)
				}
				if tag, ok := elemMap["tag"].(string); ok {
					elem.Tag = tag
				}
				if id, ok := elemMap["id"].(string); ok {
					elem.ID = id
				}
				if class, ok := elemMap["class"].(string); ok {
					elem.Class = class
				}
				if role, ok := elemMap["role"].(string); ok {
					elem.Role = role
				}
				if ce, ok := elemMap["contenteditable"].(string); ok {
					elem.ContentEditable = ce
				}
				if ph, ok := elemMap["placeholder"].(string); ok {
					elem.Placeholder = ph
				}
				if aria, ok := elemMap["aria"].(string); ok {
					elem.ARIALabel = aria
				}
				elements = append(elements, elem)
			}
		}
		return elements, nil
	}

	return nil, fmt.Errorf("unexpected result type: %T", result)
}

// ParseIframeResults parses the result from ScanIframes
func (d *Detector) ParseIframeResults(result interface{}) ([]IframeInfo, error) {
	var iframes []IframeInfo

	if arr, ok := result.([]interface{}); ok {
		for _, item := range arr {
			if iframeMap, ok := item.(map[string]interface{}); ok {
				iframe := IframeInfo{}
				if idx, ok := iframeMap["i"].(float64); ok {
					iframe.Index = int(idx)
				}
				if src, ok := iframeMap["src"].(string); ok {
					iframe.Src = src
				}
				if id, ok := iframeMap["id"].(string); ok {
					iframe.ID = id
				}
				if class, ok := iframeMap["class"].(string); ok {
					iframe.Class = class
				}
				iframes = append(iframes, iframe)
			}
		}
		return iframes, nil
	}

	return nil, fmt.Errorf("unexpected result type: %T", result)
}

// GenerateContextSwitchScript generates JavaScript to switch to an iframe context
func (d *Detector) GenerateContextSwitchScript(iframeIndex int) string {
	return fmt.Sprintf(`(() => {
		const iframe = document.querySelectorAll('iframe')[%d];
		if (!iframe) return { ok: false, error: 'iframe not found' };

		// Return the iframe's content document reference
		// Note: We'll need to use chrome-devtools-mcp's frame switching capabilities
		return {
			ok: true,
			iframeSrc: iframe.src,
			iframeId: iframe.id
		};
	})()`, iframeIndex)
}

// LogDebug logs debug messages if debug mode is enabled
func (d *Detector) LogDebug(format string, args ...interface{}) {
	if d.debug {
		fmt.Printf("[DOM Detector] "+format+"\n", args...)
	}
}

// ShouldCheckIframes determines if we should check for iframes based on main context scan
func (d *Detector) ShouldCheckIframes(mainContextElements []ElementInfo) bool {
	// If no elements found in main context, check iframes
	return len(mainContextElements) == 0
}

// ShouldCheckShadowDOM determines if we should check for Shadow DOM based on main context scan
func (d *Detector) ShouldCheckShadowDOM(mainContextElements []ElementInfo) bool {
	// If no elements found in main context, check Shadow DOM
	return len(mainContextElements) == 0
}

// FormatElementInfo formats an ElementInfo for logging
func (d *Detector) FormatElementInfo(elem ElementInfo) string {
	parts := []string{fmt.Sprintf("tag=%s", elem.Tag)}
	if elem.ID != "" {
		parts = append(parts, fmt.Sprintf("id=%s", elem.ID))
	}
	if elem.Class != "" {
		parts = append(parts, fmt.Sprintf("class=%s", truncateString(elem.Class, 50)))
	}
	if elem.Role != "" {
		parts = append(parts, fmt.Sprintf("role=%s", elem.Role))
	}
	if elem.ContentEditable != "" {
		parts = append(parts, "contenteditable=true")
	}
	if elem.Placeholder != "" {
		parts = append(parts, fmt.Sprintf("placeholder=%s", truncateString(elem.Placeholder, 30)))
	}
	if elem.ARIALabel != "" {
		parts = append(parts, fmt.Sprintf("aria-label=%s", truncateString(elem.ARIALabel, 30)))
	}
	return strings.Join(parts, ", ")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
