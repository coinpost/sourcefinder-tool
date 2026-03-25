# SourceFinder Tool - Complete Usage Example

This guide provides a complete end-to-end example of using the SourceFinder Tool, from installation to execution.

## Prerequisites

- Go 1.26.1 or later
- Node.js and npm (for chrome-devtools-mcp server)
- Chrome browser
- SourceFinder API key

## Step 1: Installation

Install the tool using Go:

```bash
go install github.com/coinpost/sourcefinder-tool@latest
```

This will install the `sourcefinder-tool` binary in your `$GOPATH/bin` directory (usually `~/go/bin/`).

**Note:** Make sure `$GOPATH/bin` is in your PATH:

```bash
# Add to your ~/.bashrc or ~/.zshrc
export PATH=$PATH:$(go env GOPATH)/bin
```

## Step 2: Prepare Files

### File 1: `user-prompt-input.md`

Create a prompt template file that defines how the AI services should process your inputs:

```markdown
Please analyze the statement (or URL) provided below, find authoritative primary sources to support or refute it, then assess its truthfulness.
Finally return the verified original source information in JSON format using the following structure:
``` json
{
    "result":true|false,
    "original_sources":[{
        "name": "name",
        "title": "title",
        "publish_date": "2026-01-01",
        "source_url": "https://source_url"
    }]
}
```

---

## Statement Details
- **Title**: ${title}
- **Content**: ${content}
- **Reference URLs**: ${source_urls}
```

**Template Placeholders:**
- `${title}` - The title field from input
- `${content}` - The content field from input
- `${source_urls}` - Comma-separated list of source URLs
- `${url}` - The URL field from input
- `${input}` - Formatted content (legacy placeholder)

### File 2: `testdata.jsonl`

Create a JSONL file (one JSON object per line) with your test data:

```json
{"url":"https://example.com/news/1","title":"Bitcoin Reaches New All-Time High","content":"Bitcoin has surpassed $150,000 for the first time in history","source_urls":["https://example.com/source1","https://example.com/source2"]}
{"url":"https://example.com/news/2","title":"Ethereum Upgrade Scheduled","content":"Ethereum's next major upgrade is scheduled for next month","source_urls":["https://ethereum.org/blog","https://example.com/crypto-news"]}
{"url":"https://example.com/news/3","title":"Test Claim Without Sources","content":"This is a test claim with no reference URLs","source_urls":[]}
```

**JSONL Format Notes:**
- Each line is a separate JSON object
- Lines starting with `#` are treated as comments
- Empty lines are ignored
- All fields are optional, but `title` and `content` are recommended
- `source_urls` should be an array (can be empty)

## Step 3: Start Chrome with CDP Enabled (Optional but Recommended)

If you want to connect to an existing Chrome instance to maintain login sessions:

**macOS:**
```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug
```

**Linux:**
```bash
google-chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug
```

**Windows:**
```cmd
"C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222 --user-data-dir="C:\chrome-debug"
```

**Note:** If you don't run this command, the tool will spawn its own Chrome instance.

## Step 4: Execute the Command

Run the following command with your actual API key:

```bash
sourcefinder-tool \
  -template user-prompt-input.md \
  -input-file testdata.jsonl \
  -sourcefinder-api-key=YOUR_ACTUAL_API_KEY \
  -site sourcefinder,chatgpt,grok \
  -browser-url http://localhost:9222 \
  --debug \
  -output check_result.json
```

**Command Parameters Explained:**

| Parameter | Description |
|-----------|-------------|
| `-template user-prompt-input.md` | Use the custom prompt template file |
| `-input-file testdata.jsonl` | Read inputs from the JSONL file |
| `-sourcefinder-api-key=YOUR_ACTUAL_API_KEY` | Your SourceFinder API key (required) |
| `-site sourcefinder,chatgpt,grok` | Run on all three sites (SourceFinder API, ChatGPT, Grok) |
| `-browser-url http://localhost:9222` | Connect to existing Chrome instance (optional) |
| `--debug` | Enable verbose logging for troubleshooting |
| `-output check_result.json` | Save results to a JSON file |

**Alternative Site Options:**
- `-site chatgpt` - Run on ChatGPT only
- `-site grok` - Run on Grok only
- `-site sourcefinder` - Run on SourceFinder API only
- `-site all` - Run on all sites (equivalent to `sourcefinder,chatgpt,grok`)
- `-site chatgpt,grok` - Run on ChatGPT and Grok (no API)

## Step 5: Expected Output

### Console Output (with --debug enabled)

```
[DEBUG] Starting SourceFinder Tool
[DEBUG] Loaded template from user-prompt-input.md
[DEBUG] Parsed 3 inputs from testdata.jsonl
[DEBUG] Sites to process: [sourcefinder chatgpt grok]
[DEBUG] Starting Phase 1: Opening browser tabs...
[DEBUG] Opened ChatGPT page (ID: 0)
[DEBUG] Opened Grok page (ID: 1)
[DEBUG] Starting Phase 2: Submitting inputs...
[DEBUG] Input 0 submitted to ChatGPT (page ID: 0)
[DEBUG] Input 0 submitted to Grok (page ID: 1)
[DEBUG] Calling SourceFinder API for input 0...
[DEBUG] Starting Phase 3: Waiting for responses...
[DEBUG] SourceFinder response received: job_id=507f3f9a-1234-5678-9abc-def123456789
[DEBUG] ChatGPT response received for input 0
[DEBUG] Grok response received for input 0
...
```

### Output Files

**Important:** When processing multiple inputs from an input file, the tool creates **one separate output file per input**, not a single combined file.

**File Naming Pattern:**
```
{outputfilename}_{title}.{outputext}
```

For the command `-output check_result.json` with testdata.jsonl containing 3 entries, you will get:

```
check_result_Bitcoin Reaches New All-Time High.json
check_result_Ethereum Upgrade Scheduled.json
check_result_Test Claim Without Sources.json
```

**Notes:**
- The title is sanitized for filenames (invalid characters like `/`, `:`, `*`, etc. are replaced with `_`)
- Title is truncated to 50 characters if too long
- If no title exists, falls back to using the input index: `check_result_0.json`

**Example Output File Content (`check_result_Bitcoin Reaches New All-Time High.json`):**

```json
=== Input 0 ===
URL: https://example.com/news/1
Title: Bitcoin Reaches New All-Time High
Content: Bitcoin has surpassed $150,000 for the first time in history

--- sourcefinder (job_id:507f3f9a-1234-5678-9abc-def123456789) ---
{
  "result": false,
  "original_sources": [
    {
      "name": "CryptoNews Network",
      "title": "Bitcoin Trading at $98,000 - No Record High Yet",
      "publish_date": "2026-03-25",
      "source_url": "https://cryptonews.example.com/bitcoin-price"
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

## Troubleshooting

### Chrome Connection Issues

If you see connection errors with `-browser-url`:
1. Verify Chrome is running with remote debugging: `curl http://localhost:9222/json/version`
2. Check that the port (9222) matches
3. Ensure no other Chrome instances are using the same port

### SourceFinder API Errors

If SourceFinder returns authentication errors:
- Verify your API key is correct: `-sourcefinder-api-key=YOUR_KEY`
- Check that the API endpoint is accessible
- Ensure you have sufficient API quota

### Timeout Issues

If requests timeout:
- Increase timeout: `-timeout 3m` (default is 2 minutes per input)
- Check your internet connection
- Verify the AI services are operational

## Additional Examples

### Quick Test (Single Input)

```bash
sourcefinder-tool \
  -text "Bitcoin reaches $150,000" \
  -site chatgpt \
  -debug
```

### Batch Processing with Plain Text File

```bash
# Create inputs.txt with one claim per line
cat > inputs.txt << EOF
Bitcoin reaches new all-time high
Ethereum scheduled upgrade next month
SEC announces new crypto regulations
EOF

# Process the file
sourcefinder-tool -input-file inputs.txt -site all -sourcefinder-api-key=YOUR_KEY
```

### Custom SourceFinder Configuration

```bash
sourcefinder-tool \
  -input-file testdata.jsonl \
  -site sourcefinder \
  -sourcefinder-api-key=YOUR_KEY \
  -sourcefinder-command "url=https://api.sourcefinder.ai --engines=google,twitter --max-results=10 --model=gpt-5"
```
