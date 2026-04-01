# SourceFinder Tool

A CLI tool for automating AI service interactions. Supports ChatGPT, Grok, and SourceFinder API via Chrome DevTools Protocol. Batch process inputs for fact-checking and source verification.

## Features

- Multi-site support: Process inputs on ChatGPT, Grok, SourceFinder API, or all simultaneously
- Structured input format: JSON-based input with URL, title, content, and source URLs
- Batch processing: Handle multiple inputs with parallel execution
- Template-based prompts: Use customizable prompt templates with multiple placeholders
- Flexible input methods: Command-line flags, input files (JSON or plain text), or positional arguments
- JSON output: Structured output with automatic JSON formatting
- Job ID tracking: SourceFinder results include job IDs for reference
- Debug mode: Verbose logging and optional screenshots for troubleshooting
- Chrome DevTools integration: Uses MCP server for browser automation

## Prerequisites

- Go 1.26.1 or later
- Node.js and npm (for chrome-devtools-mcp server)
- Chrome browser (or connect to existing Chrome instance)
- SourceFinder API key (for sourcefinder site)

## Installation

### Build from source

```bash
go build -o sourcefinder-tool main.go
```

### Using pre-built binaries

Download the latest release for your platform from the releases page.

## Usage

### Basic Examples

```bash
# Process a single input on ChatGPT
./sourcefinder-tool -text "Bitcoin reaches $150,000"

# Process multiple inputs
./sourcefinder-tool -text "Claim 1" -text "Claim 2" -text "Claim 3"

# Process on SourceFinder API
./sourcefinder-tool -text "Your input" -site sourcefinder -sourcefinder-api-key YOUR_API_KEY

# Process on both ChatGPT and Grok
./sourcefinder-tool -text "Your input" -site chatgpt,grok

# Process on all sites (ChatGPT, Grok, SourceFinder)
./sourcefinder-tool -input-file inputs.json -site all -sourcefinder-api-key YOUR_API_KEY
```

### Input File Format

#### JSON Format (Recommended)

Create a JSON file with one JSON object per line. Each line supports structured fields:

```json
{"url": "https://coinpost.ai/en/topics/90204", "title": "Binance to Revise Margin Collateral", "content": "The exchange will adjust portfolio margin rates for LRC and QTUM on Feb 27, 2026", "source_urls": ["https://www.odaily.news/zh-CN/newsflash/469369"]}
{"url": "https://example.com/news/2", "title": "Bitcoin Reaches New All-Time High", "content": "Bitcoin has surpassed $150,000 for the first time in history", "source_urls": []}
# This is a comment line and will be ignored
{"url": "", "title": "Ethereum Upgrade Scheduled", "content": "Ethereum's next upgrade is scheduled for next month", "source_urls": ["https://ethereum.org/blog"]}
```

**Field Descriptions:**
- `url` (string): The URL of the source article (optional)
- `title` (string): The title of the article or claim (optional but recommended)
- `content` (string): The main content or claim to verify (optional but recommended)
- `source_urls` (array): List of reference source URLs (optional)

#### Plain Text Format (Backward Compatible)

You can also use plain text format (one per line). Lines starting with `#` are treated as comments:

```text
# This is a comment
# Each line below is a separate input

SEC and CFTC Issue Joint Guidance on crypto regulations.

Bitcoin reaches new all-time high of $150,000 in March 2026.

Ethereum Foundation announces major protocol upgrade scheduled for Q2 2026.
```

### Custom Prompt Template

Use the `-template` flag to specify a custom prompt template file:

```bash
./sourcefinder-tool -text "Your claim" -template my-template.md
```

The template supports multiple placeholders:

```markdown
## Task
Verify the following claim and find authoritative sources:

**Title**: ${title}
**Content**: ${content}
**Source**: ${url}
**References**: ${source_urls}

Please provide a fact-check in JSON format.
```

**Available Placeholders:**
- `${url}` - The URL field from input
- `${title}` - The title field from input
- `${content}` - The content field from input
- `${source_urls}` - Comma-separated list of source URLs
- `${input}` - Legacy placeholder (formats as "title: content" if title exists)

### Output Options

By default, results are printed to stdout. Use `-output` to save to a file:

```bash
./sourcefinder-tool -input-file inputs.json -output results.json
```

**Output Format:**

```
=== Input 0 ===
URL: https://coinpost.ai/en/topics/90204
Title: Binance to Revise Margin Collateral
Content: The exchange will adjust portfolio margin rates...

--- sourcefinder (job_id:507f3f9a-1234-5678-9abc-def123456789) ---
{
  "result": false,
  "original_sources": [
    {
      "name": "NBC News",
      "title": "House Democrats to bring Epstein survivors to Trump State of Union speech",
      "publish_date": "2026-02-24",
      "source_url": "https://www.nbcnews.com/politics/congress/house-democrats-bringing-jeffrey-epstein-survivors-trumps-state-union-rcna260285"
    }
  ]
}

--- chatgpt ---
{
  "result": false,
  "original_sources": [...]
}

--- grok ---
{
  "result": false,
  "original_sources": [...]
}
```

### SourceFinder Configuration

Configure SourceFinder API parameters:

```bash
./sourcefinder-tool -input-file inputs.json -site sourcefinder \
  -sourcefinder-api-key YOUR_API_KEY \
  -sourcefinder-command "url=https://api.sourcefinder.ai --engines=google,twitter --max-results=10 --model=gpt-5"
```

**SourceFinder Options:**
- `url`: SourceFinder API endpoint (default: https://sourcefinder-api.coinpost.ai)
- `engines`: Search engines to use (default: google,twitter)
- `max-results`: Maximum number of results (default: 5)
- `model`: AI model to use (default: gpt-5)

### Debug Mode

Enable verbose logging and optionally save screenshots:

```bash
# Enable debug logging
./sourcefinder-tool -text "Your input" -debug

# Save screenshots to a directory
./sourcefinder-tool -text "Your input" -debug -screenshot-dir ./screenshots
```

### Connecting to Existing Chrome (CDP Protocol)

You can connect the tool to an existing Chrome instance using the Chrome DevTools Protocol (CDP) via the `--browserUrl` parameter. This is useful when you want to use your own Chrome profile, maintain login sessions, or have better control over the browser environment.

#### Starting Chrome with CDP Enabled

**Windows:**

```cmd
# For Chrome
"C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222 --user-data-dir="C:\chrome-debug"

# For Chrome Portable (if installed in different location)
chrome.exe --remote-debugging-port=9222 --user-data-dir="C:\chrome-debug"
```

**macOS:**

```bash
# For Chrome
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug

# Or using open command
open -a "Google Chrome" --args --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug
```

**Linux:**

```bash
# For Chrome
google-chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug

# For Chromium
chromium --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug

# For Chrome (alternative path)
/usr/bin/google-chrome-stable --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug
```

**Important Parameters:**
- `--remote-debugging-port=9222`: Enables CDP on the specified port (default: 9222)
- `--user-data-dir`: Specifies a separate user data directory to avoid conflicts with your main Chrome profile

#### Connecting to Chrome

Once Chrome is running with remote debugging enabled, connect the tool:

```bash
# Connect to local Chrome instance
./sourcefinder-tool -text "Your input" -browser-url http://localhost:9222

# Connect to remote Chrome instance
./sourcefinder-tool -text "Your input" -browser-url http://192.168.1.100:9222

# Use with other options
./sourcefinder-tool -input-file inputs.json -browser-url http://localhost:9222 -output results.json
```

**Benefits of Using `--browserUrl`:**
- Maintain login sessions across runs
- Use your existing Chrome extensions
- Better control over browser settings
- Useful for development and debugging

## Command-Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-text` | Input text to process (can be specified multiple times) | - |
| `-input-file` | File containing multiple inputs (JSON or plain text) | - |
| `-template` | Path to prompt template file | `user-prompt-input.md` |
| `-timeout` | Time to wait for response (e.g., 30s, 2m) | `120s` |
| `-site` | Target site: chatgpt, grok, sourcefinder, all, or comma-separated list | `chatgpt` |
| `-sourcefinder-api-key` | SourceFinder API key (required for sourcefinder site) | - |
| `-sourcefinder-command` | SourceFinder command parameters | `url=https://sourcefinder-api.coinpost.ai --engines=google,twitter --max-results=5 --model=gpt-5` |
| `-mcp-command` | Command to spawn MCP server | `npx -y chrome-devtools-mcp@latest` |
| `-browser-url` | Chrome DevTools Protocol endpoint | - |
| `-debug` | Enable debug logging | `false` |
| `-screenshot-dir` | Directory to save debug screenshots | - |
| `-output` | JSON output file (default: stdout) | - |

## How It Works

The tool operates in parallel phases:

1. **Phase 1**: Opens a new browser tab for each browser input-site combination (ChatGPT, Grok)
2. **Phase 2**: Sequentially inputs and submits prompts (to avoid keyboard/click conflicts)
3. **Phase 3**: Parallel waiting for all responses (browser and API)

**Parallel Execution:**
- SourceFinder API tasks and browser tasks (ChatGPT/Grok) run in parallel
- This maximizes throughput by utilizing both API and browser resources simultaneously

**Architecture:**
- Browser sites (ChatGPT, Grok): Three-phase approach (open, submit, wait)
- API sites (SourceFinder): Direct API calls with job ID tracking
- Results are collected and formatted with job IDs for reference


