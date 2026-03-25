# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based browser automation tool that interacts with AI services (ChatGPT and Grok) via Chrome DevTools Protocol (CDP). It batch processes multiple inputs across multiple AI services and collects their responses for fact-checking and source verification tasks.

## Build and Run Commands

```bash
# Build from source
go build -o sourcefinder-tool main.go

# Run with single input
./sourcefinder-tool -text "Bitcoin reaches $150,000"

# Run with multiple inputs
./sourcefinder-tool -text "Claim 1" -text "Claim 2" -text "Claim 3"

# Run with input file (one per line, # for comments)
./sourcefinder-tool -input-file inputs.txt

# Run on Grok instead of ChatGPT
./sourcefinder-tool -text "Your input" -site grok

# Run on both ChatGPT and Grok
./sourcefinder-tool -text "Your input" -site all

# Custom template
./sourcefinder-tool -text "Your claim" -template my-template.md

# Save output to file
./sourcefinder-tool -input-file inputs.txt -output results.json

# Debug mode with verbose logging
./sourcefinder-tool -text "Your input" -debug

# Connect to existing Chrome instance (CDP)
./sourcefinder-tool -text "Your input" -browser-url http://localhost:9222
```

## High-Level Architecture

The codebase follows a modular architecture with clear separation of concerns:

### Core Processing Flow (main.go)

The tool operates in a **three-phase architecture** for batch processing:

1. **Phase 1**: Open all browser tabs sequentially (one for each input-site combination)
2. **Phase 2**: Sequential input submission (to avoid keyboard/click conflicts between tabs)
3. **Phase 3**: Sequential response waiting (CDP uses single connection; parallel SelectPage calls interfere)

This design ensures reliable input submission while maximizing throughput for response collection. The sequential requirement in Phases 2 and 3 is critical because `chrome-devtools-mcp` uses a single stdio connection, causing tool calls to be serialized.

### Package Structure

- **config/**: CLI flag parsing and configuration validation (`-text`, `-input-file`, `-site`, `-timeout`, etc.)
- **mcp/**: Chrome DevTools Protocol client wrapper using `chrome-devtools-mcp` server
  - `client.go`: MCP client initialization and connection management
  - `tools.go`: High-level Chrome automation methods (NavigatePage, Click, Fill, EvaluateScript, SelectPage, ListPages)
- **chatgpt/**: ChatGPT-specific automation logic
  - `selectors.go`: DOM element selectors and response extraction scripts
  - `types.go`: Response data structures
  - `automation.go`: Workflow implementation (InputAndSubmitOnly, WaitForResponse)
- **grok/**: Grok-specific automation logic (parallel structure to chatgpt)
  - `selectors.go`, `types.go`, `automation.go`, `smart_finder.go`
- **dom/**: Generic DOM interaction utilities
  - `button_finder.go`: Multi-strategy send button detection and clicking
  - `input_finder.go`: Input element detection strategies
  - `detector.go`: Page state detection utilities
- **document/**: Document reading utilities (currently minimal)

### Key Design Patterns

**Template Processing**: The tool reads a markdown template (`user-prompt-input.md` by default) and replaces `${input}` with actual user input. The markdown is converted to HTML for both ChatGPT and Grok inputs (main.go:markdownToHTML).

**Site Abstraction**: Both ChatGPT and Grok implement the same Agent interface pattern:
- `InputAndSubmitOnly`: Assumes page is already loaded, fills input and submits
- `WaitForResponse`: Polls for response completion and extracts content

**Multi-Tab Coordination**: The main.go:processInputs function orchestrates multiple browser tabs using:
- `cdt.NewPage()` to open tabs
- `cdt.ListPages()` to get page IDs
- `cdt.SelectPage(pageId, false)` to switch context before each operation

### Response Extraction

Both sites poll for response completion using JavaScript:

- **ChatGPT**: Looks for `div.cm-content` elements, extracts the LAST one (most recent response)
- **Grok**: Two-phase polling (element appearance, then JSON validation)

### Important Implementation Details

1. **Sequential Phases 2 & 3 are intentional**: Despite seeming inefficient, this is required because the MCP protocol serializes all tool calls through a single connection. Parallel goroutines cause SelectPage calls to interfere.

2. **HTML Injection for Prompts**: ChatGPT uses `innerHTML` to inject formatted HTML (chatgpt/selectors.go:FillInputWithHTMLScript), while Grok uses a smart finder that tries multiple strategies.

3. **Page Selection**: Always call `cdt.SelectPage(ctx, pageId, false)` before operating on a specific tab. The `bringToFront=false` parameter avoids unnecessary tab switching.

4. **Input File Format**: Lines starting with `#` are treated as comments. Empty lines are skipped.

5. **Timeout Management**: Each input gets a fresh timeout context. The default is 120 seconds per input.

## Dependencies

- `github.com/mark3labs/mcp-go`: MCP (Model Context Protocol) Go client for communicating with chrome-devtools-mcp server
- Node.js and `npx`: Required to spawn the `chrome-devtools-mcp@latest` server (or connect to existing Chrome via CDP)

## Adding Support for New AI Sites

To add a new AI service (e.g., Claude, Perplexity):

1. Create a new package (e.g., `claude/`) mirroring the structure of `chatgpt/` or `grok/`
2. Implement the same Agent interface with:
   - `NewAgent(cdt *mcp.ChromeDevTools, timeout time.Duration, debug bool) *Agent`
   - `InputAndSubmitOnly(ctx, prompt string) error`
   - `WaitForResponse(ctx) (*Response, error)`
3. Add selectors in `selectors.go` for input detection and response extraction
4. Update `config.go` to add the site name to valid options
5. Update `main.go:getSiteURL()` to return the new site's URL
6. Add the site to the site selection logic in `main.go:processInputs()`
