package grok

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coinpost/sourcefinder-tool/dom"
	"github.com/coinpost/sourcefinder-tool/mcp"
)

// SmartInputFinder implements the recommended input finding strategy
// Step 1: Scan main DOM
// Step 2: Check iframes if needed
// Step 3: Check Shadow DOM if needed
type SmartInputFinder struct {
	cdt          *mcp.ChromeDevTools
	inputFinder  *dom.InputFinder
	buttonFinder *dom.ButtonFinder
	debug        bool
}

// NewSmartInputFinder creates a new smart input finder
func NewSmartInputFinder(cdt *mcp.ChromeDevTools, debug bool) *SmartInputFinder {
	return &SmartInputFinder{
		cdt:          cdt,
		inputFinder:  dom.NewInputFinder(debug),
		buttonFinder: dom.NewButtonFinder(debug),
		debug:        debug,
	}
}

// FindAndFillInput implements the complete input finding and filling strategy
func (f *SmartInputFinder) FindAndFillInput(ctx context.Context, text string) error {
	if f.debug {
		log.Printf("[SmartFinder] ============================================")
		log.Printf("[SmartFinder] Starting smart input search for %d chars of text", len(text))
		log.Printf("[SmartFinder] ============================================")
	}

	// Step 1: Scan for input elements in main DOM
	if f.debug {
		log.Printf("[SmartFinder] Step 1: Scanning main DOM for input elements...")
	}

	scanScript := f.inputFinder.ScanForInputElements()
	if f.debug {
		log.Printf("[SmartFinder] Executing scan script...")
	}

	scanResult, err := f.cdt.EvaluateScript(ctx, scanScript)
	if err != nil {
		if f.debug {
			log.Printf("[SmartFinder] ERROR: Scan script execution failed: %v", err)
		}
		return fmt.Errorf("scan script failed: %w", err)
	}

	if f.debug {
		scanJSON, _ := json.MarshalIndent(scanResult, "", "  ")
		log.Printf("[SmartFinder] ✓ Scan completed successfully")
		log.Printf("[SmartFinder] Raw scan result:\n%s", string(scanJSON))
	}

	// Analyze scan results
	if f.debug {
		log.Printf("[SmartFinder] Analyzing scan results...")
	}
	analysis, err := f.inputFinder.AnalyzeScanResults(scanResult)
	if err != nil {
		if f.debug {
			log.Printf("[SmartFinder] ERROR: Failed to analyze results: %v", err)
		}
		return fmt.Errorf("failed to analyze scan results: %w", err)
	}

	if f.debug {
		log.Printf("[SmartFinder] ✓ Analysis complete")
		log.Printf("[SmartFinder] Recommended action: %s", analysis.GetRecommendedAction())
		log.Printf("[SmartFinder] Details:")
		log.Printf("[SmartFinder]   - Available in main: %v", analysis.AvailableInMain)
		log.Printf("[SmartFinder]   - Main DOM elements: %d", analysis.MainDOMElementCount)
		log.Printf("[SmartFinder]   - Has iframes: %v (%d)", analysis.HasIframes, analysis.IframeCount)
		log.Printf("[SmartFinder]   - Has Shadow DOM: %v (%d)", analysis.HasShadowDOM, analysis.ShadowHostCount)
		log.Printf("[SmartFinder]   - Recommended selector: %s", analysis.RecommendedSelector)
	}

	// Step 2-3: Handle different contexts
	if analysis.AvailableInMain {
		// Input found in main DOM - use it directly
		if f.debug {
			log.Printf("[SmartFinder] ✓ Input found in main DOM, proceeding to fill...")
		}
		return f.fillInputInMainDOM(ctx, text)
	}

	// Check if we need to look in iframes
	if analysis.HasIframes {
		if f.debug {
			log.Printf("[SmartFinder] Step 2: Checking iframes (%d found)...", analysis.IframeCount)
		}

		if analysis.HasSameOriginIframes {
			// Try to find and fill input in iframe
			// Note: This requires frame switching capability in chrome-devtools-mcp
			if f.debug {
				log.Printf("[SmartFinder] Found %d same-origin iframes, but frame switching not yet implemented", analysis.IframeCount)
			}
		} else {
			return fmt.Errorf("input likely in cross-origin iframe (CORS prevents access)")
		}
	}

	// Check Shadow DOM
	if analysis.HasShadowDOM {
		if f.debug {
			log.Printf("[SmartFinder] Step 3: Checking Shadow DOM (%d hosts found)...", analysis.ShadowHostCount)
		}

		if analysis.HasInputInShadowDOM {
			return f.fillInputInShadowDOM(ctx, text)
		}
	}

	// If all else fails, try a comprehensive diagnostic
	return f.runDiagnostics(ctx, text)
}

// fillInputInMainDOM fills the input in main DOM context
func (f *SmartInputFinder) fillInputInMainDOM(ctx context.Context, text string) error {
	if f.debug {
		log.Printf("[SmartFinder] --------------------------------------------")
		log.Printf("[SmartFinder] Filling input in main DOM context")
		log.Printf("[SmartFinder] Text length: %d chars", len(text))
		log.Printf("[SmartFinder] --------------------------------------------")
	}

	// Use the input finder's script to find and fill
	fillScript := f.inputFinder.FindInputScript(text)
	if f.debug {
		log.Printf("[SmartFinder] Executing fill script...")
		// Show first 200 chars of the script for debugging
		scriptPreview := fillScript
		if len(scriptPreview) > 200 {
			scriptPreview = scriptPreview[:200] + "..."
		}
		log.Printf("[SmartFinder] Script preview: %s", scriptPreview)
	}

	result, err := f.cdt.EvaluateScript(ctx, fillScript)
	if err != nil {
		if f.debug {
			log.Printf("[SmartFinder] ERROR: Fill script execution failed: %v", err)
		}
		return fmt.Errorf("fill input script failed: %w", err)
	}

	if f.debug {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		log.Printf("[SmartFinder] ✓ Fill script executed")
		log.Printf("[SmartFinder] Result:\n%s", string(resultJSON))
	}

	// Check if successful
	if resultMap, ok := result.(map[string]interface{}); ok {
		if ok, exists := resultMap["ok"].(bool); exists && ok {
			if f.debug {
				log.Printf("[SmartFinder] ✓✓ Successfully filled input!")
				log.Printf("[SmartFinder]    - Selector: %v", resultMap["selector"])
				log.Printf("[SmartFinder]    - Tag: %v", resultMap["tag"])
				log.Printf("[SmartFinder]    - Method: %v", resultMap["method"])
			}
			return nil
		}
		if errMsg, exists := resultMap["error"].(string); exists {
			if f.debug {
				log.Printf("[SmartFinder] ERROR: Fill failed with message: %s", errMsg)
			}
			return fmt.Errorf("fill input failed: %s", errMsg)
		}
	}

	if f.debug {
		log.Printf("[SmartFinder] ERROR: Unexpected fill result format: %v", result)
	}
	return fmt.Errorf("unexpected fill result: %v", result)
}

// fillInputInShadowDOM fills the input in Shadow DOM
func (f *SmartInputFinder) fillInputInShadowDOM(ctx context.Context, text string) error {
	if f.debug {
		log.Printf("[SmartFinder] Using Shadow DOM context to fill input")
	}

	// Check if text contains HTML tags
	containsHTML := strings.Contains(text, "<") && strings.Contains(text, ">")

	shadowScript := fmt.Sprintf(`() => {
		const textContent = %q;
		const containsHTML = %t;

		// Search all Shadow DOM hosts
		for (const host of document.querySelectorAll('*')) {
			if (!host.shadowRoot) continue;

			const input = host.shadowRoot.querySelector(
				'[contenteditable="true"], textarea, [role="textbox"], input[type="text"]'
			);

			if (input) {
				const rect = input.getBoundingClientRect();
				if (rect.width > 0 && rect.height > 0) {
					input.focus();

					const isContentEditable = input.getAttribute('contenteditable') === 'true' ||
					                          input.getAttribute('contenteditable') === '' ||
					                          input.contentEditable === 'true';

					if (isContentEditable) {
						if (containsHTML) {
							input.innerHTML = textContent;
						} else {
							input.textContent = textContent;
						}
						input.dispatchEvent(new InputEvent('input', { bubbles: true }));
						return {
							ok: true,
							context: 'shadow_dom',
							hostTag: host.tagName,
							inputTag: input.tagName,
							method: containsHTML ? 'innerHTML' : 'textContent',
							contentEditableAttr: input.getAttribute('contenteditable')
						};
					} else if (input.value !== undefined) {
						input.value = textContent;
						input.dispatchEvent(new Event('input', { bubbles: true }));
						return {
							ok: true,
							context: 'shadow_dom',
							hostTag: host.tagName,
							inputTag: input.tagName,
							method: 'value',
							warning: 'textarea/input only supports plain text, HTML tags will be displayed as text'
						};
					}
				}
			}
		}

		return { ok: false, error: 'No input found in Shadow DOM' };
	}`, text, containsHTML)

	result, err := f.cdt.EvaluateScript(ctx, shadowScript)
	if err != nil {
		return fmt.Errorf("shadow DOM script failed: %w", err)
	}

	if resultMap, ok := result.(map[string]interface{}); ok {
		if ok, exists := resultMap["ok"].(bool); exists && ok {
			if f.debug {
				log.Printf("[SmartFinder] Successfully filled input in Shadow DOM")
			}
			return nil
		}
		if errMsg, exists := resultMap["error"].(string); exists {
			return fmt.Errorf("shadow DOM fill failed: %s", errMsg)
		}
	}

	return fmt.Errorf("unexpected shadow DOM result: %v", result)
}

// runDiagnostics runs comprehensive diagnostics when standard methods fail
func (f *SmartInputFinder) runDiagnostics(ctx context.Context, _ string) error {
	if f.debug {
		log.Printf("[SmartFinder] Running comprehensive diagnostics...")
	}

	// Diagnostic script to gather all possible input-related information
	diagnosticScript := `() => {
		const result = {
			url: window.location.href,
			readyState: document.readyState,

			// All possible input elements
			inputs: [...document.querySelectorAll('input')].map(el => ({
				type: el.type,
				id: el.id,
				class: el.className,
				visible: el.offsetParent !== null,
				placeholder: el.placeholder
			})),

			textareas: [...document.querySelectorAll('textarea')].map(el => ({
				id: el.id,
				class: el.className,
				visible: el.offsetParent !== null,
				placeholder: el.placeholder
			})),

			contentEditables: [...document.querySelectorAll('[contenteditable="true"]')].map(el => ({
				tag: el.tagName,
				id: el.id,
				class: el.className,
				visible: el.offsetParent !== null
			})),

			roleTextboxes: [...document.querySelectorAll('[role="textbox"]')].map(el => ({
				tag: el.tagName,
				id: el.id,
				class: el.className,
				visible: el.offsetParent !== null
			})),

			// Count iframes
			iframeCount: document.querySelectorAll('iframe').length,

			// Count Shadow DOM hosts
			shadowHostCount: [...document.querySelectorAll('*')].filter(el => el.shadowRoot).length,

			// All classes and IDs (to help identify patterns)
			allClasses: [...new Set([...document.querySelectorAll('*')].map(el => el.className).filter(c => c))].slice(0, 50),
			allIds: [...new Set([...document.querySelectorAll('[id]')].map(el => el.id))].slice(0, 50)
		};

		return result;
	}`

	diagResult, err := f.cdt.EvaluateScript(ctx, diagnosticScript)
	if err != nil {
		return fmt.Errorf("diagnostics failed: %w", err)
	}

	// Pretty print diagnostics
	diagJSON, _ := json.MarshalIndent(diagResult, "", "  ")
	log.Printf("[SmartFinder] DIAGNOSTICS:\n%s", string(diagJSON))

	// Save diagnostics to file for analysis
	if err := saveDiagnostics(diagResult); err != nil {
		log.Printf("[SmartFinder] Warning: failed to save diagnostics: %v", err)
	}

	return fmt.Errorf("no suitable input element found after comprehensive search")
}

// SubmitInput submits the filled input using the most reliable method
func (f *SmartInputFinder) SubmitInput(ctx context.Context) error {
	if f.debug {
		log.Printf("[SmartFinder] Attempting to submit input...")
	}

	// IMPORTANT: For Grok and similar sites, the submit button may only appear after typing
	// Wait a moment for the button to appear
	if f.debug {
		log.Printf("[SmartFinder] Waiting for submit button to appear...")
	}
	time.Sleep(1 * time.Second)

	// Strategy 1: Click send button (for Grok, this is more reliable than Enter)
	// Grok's button: <button type="submit" aria-label="提交" tabindex="0">
	if f.debug {
		log.Printf("[SmartFinder] Strategy 1: Scanning for submit button...")
	}

	// First, scan for the button to see what's available
	scanScript := f.buttonFinder.ScanForButtons()
	scanResult, err := f.cdt.EvaluateScript(ctx, scanScript)
	if err == nil && f.debug {
		scanJSON, _ := json.MarshalIndent(scanResult, "", "  ")
		scanStr := string(scanJSON)
		if len(scanStr) > 1000 {
			scanStr = scanStr[:1000] + "..."
		}
		log.Printf("[SmartFinder] Button scan result: %s", scanStr)
	}

	// Try to click the button
	clickScript := f.buttonFinder.ClickSendButtonScript()
	result, err := f.cdt.EvaluateScript(ctx, clickScript)
	if err == nil {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if ok, exists := resultMap["ok"].(bool); exists && ok {
				if f.debug {
					log.Printf("[SmartFinder] Successfully submitted by clicking button: %v", resultMap["method"])
				}
				return nil
			}
		}
	}

	if f.debug {
		log.Printf("[SmartFinder] Button click failed, trying Strategy 2: Press Enter key...")
	}

	// Strategy 2: Press Enter (backup method)
	enterScript := f.buttonFinder.PressEnterScript()
	result, err = f.cdt.EvaluateScript(ctx, enterScript)
	if err == nil {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if ok, exists := resultMap["ok"].(bool); exists && ok {
				if f.debug {
					log.Printf("[SmartFinder] Successfully submitted with Enter key")
				}
				return nil
			}
		}
	}

	return fmt.Errorf("all submit strategies failed")
}

// WaitForInputReady waits for the page to be ready for input
func (f *SmartInputFinder) WaitForInputReady(ctx context.Context, timeout time.Duration) error {
	if f.debug {
		log.Printf("[SmartFinder] ============================================")
		log.Printf("[SmartFinder] Waiting for input to be ready")
		log.Printf("[SmartFinder] Timeout: %v", timeout)
		log.Printf("[SmartFinder] ============================================")
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond
	attempts := 0

	checkScript := `() => {
		// Check if ANY input element exists and is visible
		const inputs = document.querySelectorAll(
			'textarea, input, [contenteditable="true"], [role="textbox"]'
		);

		const visibleInputs = [];
		for (const input of inputs) {
			const rect = input.getBoundingClientRect();
			if (rect.width > 0 && rect.height > 0) {
				visibleInputs.push({
					tag: input.tagName,
					type: input.type || 'N/A',
					contenteditable: input.contentEditable,
					placeholder: input.placeholder || '',
					ariaLabel: input.getAttribute('aria-label') || ''
				});
			}
		}

		return {
			ready: visibleInputs.length > 0,
			found: inputs.length,
			visible: visibleInputs.length,
			visibleInputs: visibleInputs
		};
	}`

	for time.Now().Before(deadline) {
		attempts++
		if f.debug && attempts == 1 {
			log.Printf("[SmartFinder] Attempt %d: Checking for visible inputs...", attempts)
		}

		result, err := f.cdt.EvaluateScript(ctx, checkScript)
		if err != nil {
			if f.debug {
				log.Printf("[SmartFinder] WARNING: Check script failed: %v", err)
			}
		} else {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if ready, ok := resultMap["ready"].(bool); ok && ready {
					if f.debug {
						log.Printf("[SmartFinder] ✓✓ Input is ready!")
						log.Printf("[SmartFinder]    - Total inputs found: %v", resultMap["found"])
						log.Printf("[SmartFinder]    - Visible inputs: %v", resultMap["visible"])
						if visibleInputs, ok := resultMap["visibleInputs"].([]interface{}); ok && len(visibleInputs) > 0 {
							for i, inp := range visibleInputs {
								if inpMap, ok := inp.(map[string]interface{}); ok {
									log.Printf("[SmartFinder]    - Input %d: tag=%v, type=%v, contenteditable=%v",
										i, inpMap["tag"], inpMap["type"], inpMap["contenteditable"])
								}
							}
						}
					}
					return nil
				}
				if f.debug && attempts%4 == 0 { // Log every 2 seconds
					log.Printf("[SmartFinder] Still waiting... (attempt %d, found %v inputs, %v visible)",
						attempts, resultMap["found"], resultMap["visible"])
				}
			}
		}

		select {
		case <-time.After(pollInterval):
			// Continue polling
		case <-ctx.Done():
			if f.debug {
				log.Printf("[SmartFinder] ERROR: Context cancelled while waiting")
			}
			return ctx.Err()
		}
	}

	if f.debug {
		log.Printf("[SmartFinder] ERROR: Timeout after %d attempts", attempts)
	}
	return fmt.Errorf("timeout waiting for input to be ready after %d attempts", attempts)
}

// saveDiagnostics saves diagnostic information to a file
func saveDiagnostics(data interface{}) error {
	diagJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("diagnostics_%d.json", time.Now().Unix())
	if err := json.Unmarshal(diagJSON, &data); err != nil {
		return err
	}

	log.Printf("[SmartFinder] Diagnostics saved to: %s", filename)
	return nil
}
