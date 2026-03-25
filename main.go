package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coinpost/sourcefinder-tool/chatgpt"
	"github.com/coinpost/sourcefinder-tool/config"
	"github.com/coinpost/sourcefinder-tool/grok"
	"github.com/coinpost/sourcefinder-tool/mcp"
	"github.com/coinpost/sourcefinder-tool/sourcefinder"
)

// FactCheckInput represents a structured input with all fields
type FactCheckInput struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	SourceURLs  []string `json:"source_urls"`
}

// InputSource represents a single input to process
type InputSource struct {
	Index int
	Input FactCheckInput
	Text  string // Deprecated: kept for backward compatibility
}

// Result represents the result of processing one input on one site
type Result struct {
	Index   int
	Site    string
	Success bool
	Text    string
	Error   error
	JobID   string // SourceFinder job ID (if applicable)
}

func main() {
	// Parse configuration
	cfg, err := config.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	if cfg.Debug {
		log.SetFlags(log.Ltime | log.Lshortfile)
		log.Printf("Starting agent for sites: %v", cfg.Sites)
		log.Printf("Timeout: %v", cfg.Timeout)
		if cfg.BrowserURL != "" {
			log.Printf("Browser URL: %s", cfg.BrowserURL)
		}
	}

	// Set debug flag for mcp package
	mcp.SetDebug(cfg.Debug)

	// Collect all input sources
	inputs, err := collectInputs(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting inputs: %v\n", err)
		os.Exit(1)
	}

	if len(inputs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No inputs to process\n")
		os.Exit(1)
	}

	if cfg.Debug {
		log.Printf("Collected %d input sources", len(inputs))
	}

	// Read prompt template
	templateContent, err := os.ReadFile(cfg.PromptTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading template file: %v\n", err)
		os.Exit(1)
	}

	if cfg.Debug {
		log.Printf("Template loaded from %s: %d characters", cfg.PromptTemplate, len(templateContent))
	}

	// Convert markdown to plain text for both sites
	template := markdownToText(string(templateContent))
	if cfg.Debug {
		log.Printf("Converted markdown to plain text")
	}

	// Process all inputs across all sites
	results := processInputs(cfg, inputs, template)

	// Output results
	outputResults(cfg, inputs, results)
}

// getSiteURL returns the URL for the specified site
func getSiteURL(site string) string {
	switch site {
	case "grok":
		return "https://grok.com/"
	case "chatgpt":
		return "https://chatgpt.com/"
	case "sourcefinder":
		return "" // SourceFinder doesn't use a URL
	default:
		return "https://chatgpt.com/"
	}
}

// markdownToText converts basic markdown formatting to plain text
func markdownToText(md string) string {
	lines := strings.Split(md, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			result = append(result, "")
			continue
		}

		// Remove header markers (# ## ### etc)
		for strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimSpace(trimmed[1:])
		}

		// Remove bold markers (** and __) - do this before removing single chars
		trimmed = strings.ReplaceAll(trimmed, "**", "")
		trimmed = strings.ReplaceAll(trimmed, "__", "")

		// Remove code block markers (```)
		trimmed = strings.ReplaceAll(trimmed, "```", "")

		// Remove inline code markers (`)
		trimmed = strings.ReplaceAll(trimmed, "`", "")

		// Remove blockquote markers (>)
		for strings.HasPrefix(trimmed, ">") {
			trimmed = strings.TrimSpace(trimmed[1:])
		}

		// Remove list markers (-, + at start of line, but not single * which might be part of text)
		if len(trimmed) > 0 {
			if trimmed[0] == '-' || trimmed[0] == '+' {
				trimmed = strings.TrimSpace(trimmed[1:])
			} else if trimmed[0] >= '0' && trimmed[0] <= '9' && len(trimmed) > 1 && trimmed[1] == '.' {
				// Remove numbered list markers (1., 2., etc)
				idx := 0
				for idx < len(trimmed) && trimmed[idx] >= '0' && trimmed[idx] <= '9' {
					idx++
				}
				if idx < len(trimmed) && trimmed[idx] == '.' {
					trimmed = strings.TrimSpace(trimmed[idx+1:])
				}
			}
		}

		// Remove link markdown [text](url) - keep only text
		linkStart := strings.Index(trimmed, "[")
		if linkStart != -1 {
			linkEnd := strings.Index(trimmed, "](")
			if linkEnd != -1 {
				urlEnd := strings.Index(trimmed[linkEnd+2:], ")")
				if urlEnd != -1 {
					// Extract just the link text
					linkText := trimmed[linkStart+1 : linkEnd]
					trimmed = trimmed[:linkStart] + linkText + trimmed[linkEnd+3+urlEnd:]
				}
			}
		}

		result = append(result, trimmed)
	}

	return strings.Join(result, "\n")
}

// markdownToHTML converts basic markdown to HTML for Grok input
func markdownToHTML(md string) string {
	lines := strings.Split(md, "\n")
	var html []string
	inCodeBlock := false
	inBlockquote := false
	inList := false
	var codeLines []string
	var codeBlockLang string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle code blocks
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				// Start code block
				inCodeBlock = true
				codeBlockLang = strings.TrimPrefix(trimmed, "```")
				codeLines = []string{} // Reset code lines
				continue
			} else {
				// End code block - build complete code block element
				inCodeBlock = false

				// Escape HTML entities in each line
				var escapedLines []string
				for _, codeLine := range codeLines {
					escaped := strings.ReplaceAll(codeLine, "&", "&amp;")
					escaped = strings.ReplaceAll(escaped, "<", "&lt;")
					escaped = strings.ReplaceAll(escaped, ">", "&gt;")
					escapedLines = append(escapedLines, escaped)
				}

				// Join with actual newlines
				codeContent := strings.Join(escapedLines, "\n")

				// Build complete code block
				if codeBlockLang != "" {
					html = append(html, fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>", codeBlockLang, codeContent))
				} else {
					html = append(html, fmt.Sprintf("<pre><code>%s</code></pre>", codeContent))
				}
				continue
			}
		}

		// If in code block, collect the line
		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Empty lines
		if trimmed == "" {
			// Close any open lists or blockquotes
			if inList {
				html = append(html, "</ul>")
				inList = false
			}
			if inBlockquote {
				html = append(html, "</blockquote>")
				inBlockquote = false
			}
			html = append(html, "<br>")
			continue
		}

		// Headers
		if strings.HasPrefix(trimmed, "### ") {
			html = append(html, fmt.Sprintf("<h3>%s</h3>", processInlineMarkdown(strings.TrimSpace(trimmed[4:]))))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			html = append(html, fmt.Sprintf("<h2>%s</h2>", processInlineMarkdown(strings.TrimSpace(trimmed[3:]))))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			html = append(html, fmt.Sprintf("<h1>%s</h1>", processInlineMarkdown(strings.TrimSpace(trimmed[2:]))))
			continue
		}

		// Blockquote
		if strings.HasPrefix(trimmed, "> ") {
			if !inBlockquote {
				html = append(html, "<blockquote>")
				inBlockquote = true
			}
			html = append(html, processInlineMarkdown(strings.TrimSpace(trimmed[2:])))
			continue
		} else if inBlockquote {
			html = append(html, "</blockquote>")
			inBlockquote = false
		}

		// Lists
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				html = append(html, "<ul>")
				inList = true
			}
			html = append(html, fmt.Sprintf("<li>%s</li>", processInlineMarkdown(strings.TrimSpace(trimmed[2:]))))
			continue
		}

		// Numbered lists
		if len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			if idx := strings.Index(trimmed, ". "); idx > 0 {
				if !inList {
					html = append(html, "<ol>")
					inList = true
				}
				html = append(html, fmt.Sprintf("<li>%s</li>", processInlineMarkdown(strings.TrimSpace(trimmed[idx+2:]))))
				continue
			}
		}

		// Close list if we're in one but this line isn't a list item
		if inList {
			html = append(html, "</ul>")
			inList = false
		}

		// Regular paragraph
		html = append(html, fmt.Sprintf("<p>%s</p>", processInlineMarkdown(trimmed)))
	}

	// Close any remaining open tags
	if inList {
		html = append(html, "</ul>")
	}
	if inBlockquote {
		html = append(html, "</blockquote>")
	}

	return strings.Join(html, "")
}

// processInlineMarkdown processes inline markdown (bold, italic, code, links)
func processInlineMarkdown(text string) string {
	// Inline code (`code`) - do this first to avoid processing markdown inside code
	for {
		start := strings.Index(text, "`")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+1:], "`")
		if end == -1 {
			break
		}
		end += start + 1
		code := text[start+1 : end]
		text = text[:start] + "<code>" + code + "</code>" + text[end+1:]
	}

	// Bold (**text** or __text__)
	for {
		start := strings.Index(text, "**")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		bold := text[start+2 : end]
		text = text[:start] + "<strong>" + bold + "</strong>" + text[end+2:]
	}

	// Underline bold (__text__)
	for {
		start := strings.Index(text, "__")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+2:], "__")
		if end == -1 {
			break
		}
		end += start + 2
		bold := text[start+2 : end]
		text = text[:start] + "<strong>" + bold + "</strong>" + text[end+2:]
	}

	// Links [text](url)
	for {
		start := strings.Index(text, "[")
		if start == -1 {
			break
		}
		mid := strings.Index(text[start:], "](")
		if mid == -1 {
			break
		}
		mid += start
		end := strings.Index(text[mid+2:], ")")
		if end == -1 {
			break
		}
		end += mid + 2

		linkText := text[start+1 : mid]
		linkURL := text[mid+2 : end]
		text = text[:start] + fmt.Sprintf("<a href=\"%s\">%s</a>", linkURL, linkText) + text[end+1:]
	}

	return text
}

// collectInputs collects all input sources from config
func collectInputs(cfg *config.Config) ([]InputSource, error) {
	var inputs []InputSource
	index := 0

	// Add inputs from -text flags (backward compatibility)
	for _, text := range cfg.InputTexts {
		if strings.TrimSpace(text) != "" {
			trimmed := strings.TrimSpace(text)
			inputs = append(inputs, InputSource{
				Index: index,
				Input: FactCheckInput{Content: trimmed},
				Text:  trimmed,
			})
			index++
		}
	}

	// Add inputs from file
	if cfg.InputFile != "" {
		fileInputs, err := readInputsFromFile(cfg.InputFile, index)
		if err != nil {
			return nil, fmt.Errorf("reading input file: %w", err)
		}
		inputs = append(inputs, fileInputs...)
	}

	// Handle legacy DocumentPath (for backward compatibility)
	if cfg.DocumentPath != "" && len(inputs) == 0 {
		content, err := os.ReadFile(cfg.DocumentPath)
		if err != nil {
			return nil, fmt.Errorf("reading document: %w", err)
		}
		inputs = append(inputs, InputSource{
			Index: 0,
			Input: FactCheckInput{Content: string(content)},
			Text:  string(content),
		})
	}

	return inputs, nil
}

// replaceTemplatePlaceholders replaces all placeholders in the template with actual values
func replaceTemplatePlaceholders(template string, input FactCheckInput) string {
	result := template

	// Replace ${url}
	result = strings.ReplaceAll(result, "${url}", input.URL)

	// Replace ${title}
	result = strings.ReplaceAll(result, "${title}", input.Title)

	// Replace ${content}
	result = strings.ReplaceAll(result, "${content}", input.Content)

	// Replace ${source_urls} as comma-separated list
	sourceURLsStr := strings.Join(input.SourceURLs, ", ")
	result = strings.ReplaceAll(result, "${source_urls}", sourceURLsStr)

	// Replace ${input} with formatted content (backward compatibility)
	// If title exists, use "title: content", otherwise just content
	inputText := input.Content
	if input.Title != "" {
		inputText = fmt.Sprintf("%s: %s", input.Title, input.Content)
	}
	result = strings.ReplaceAll(result, "${input}", inputText)

	return result
}

// readInputsFromFile reads inputs from a file (one JSON per line)
func readInputsFromFile(filePath string, startIndex int) ([]InputSource, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var inputs []InputSource
	index := startIndex
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") { // Skip empty lines and comments
			continue
		}

		// Try to parse as JSON
		var factCheckInput FactCheckInput
		if err := json.Unmarshal([]byte(line), &factCheckInput); err != nil {
			// If not valid JSON, treat as plain text (backward compatibility)
			inputs = append(inputs, InputSource{
				Index: index,
				Input: FactCheckInput{Content: line},
				Text:  line,
			})
			index++
			continue
		}

		// Successfully parsed JSON
		inputs = append(inputs, InputSource{
			Index: index,
			Input: factCheckInput,
			Text:  fmt.Sprintf("%s: %s", factCheckInput.Title, factCheckInput.Content),
		})
		index++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return inputs, nil
}

// InputSourceWithPage extends InputSource with page ID and site tracking
type InputSourceWithPage struct {
	InputSource
	Site   string
	PageID int
}

// Task represents a single input-site combination to process
type Task struct {
	InputIndex int
	Input      InputSource
	Site       string
	PageID     string
}

// processInputs processes all input sources across all sites
// For browser sites (chatgpt, grok): three phases (open tabs, submit, wait)
// For API sites (sourcefinder): direct API calls
func processInputs(cfg *config.Config, inputs []InputSource, template string) []Result {
	// Separate tasks into browser tasks and API tasks
	var browserTasks []Task
	var apiTasks []Task

	for i, input := range inputs {
		for _, site := range cfg.Sites {
			task := Task{
				InputIndex: i,
				Input:      input,
				Site:       site,
			}
			if site == "sourcefinder" {
				apiTasks = append(apiTasks, task)
			} else {
				browserTasks = append(browserTasks, task)
			}
		}
	}

	// Calculate total results
	totalTasks := len(browserTasks) + len(apiTasks)

	if cfg.Debug {
		log.Printf("Created %d tasks (%d browser, %d API)",
			totalTasks, len(browserTasks), len(apiTasks))
	}

	// Process API tasks and browser tasks in parallel
	var allResults []Result

	// Use channels to collect results
	apiResultsChan := make(chan []Result, 1)
	browserResultsChan := make(chan []Result, 1)

	// Process API tasks in goroutine if there are any
	if len(apiTasks) > 0 {
		if cfg.Debug {
			log.Printf("Starting %d API tasks in parallel...", len(apiTasks))
		}
		go func() {
			apiResults := processAPITasks(cfg, apiTasks, template)
			apiResultsChan <- apiResults
		}()
	} else {
		close(apiResultsChan)
	}

	// Process browser tasks in goroutine if there are any
	if len(browserTasks) > 0 {
		if cfg.Debug {
			log.Printf("Starting %d browser tasks in parallel...", len(browserTasks))
		}
		go func() {
			browserResults := processBrowserTasks(cfg, inputs, template, browserTasks)
			browserResultsChan <- browserResults
		}()
	} else {
		close(browserResultsChan)
	}

	// Collect API results
	if len(apiTasks) > 0 {
		apiResults := <-apiResultsChan
		allResults = append(allResults, apiResults...)
		if cfg.Debug {
			log.Printf("API tasks completed")
		}
	}

	// Collect browser results
	if len(browserTasks) > 0 {
		browserResults := <-browserResultsChan
		allResults = append(allResults, browserResults...)
		if cfg.Debug {
			log.Printf("Browser tasks completed")
		}
	}

	return allResults
}

// processAPITasks handles API-based tasks (sourcefinder)
func processAPITasks(cfg *config.Config, tasks []Task, template string) []Result {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout+120*time.Second)
	defer cancel()

	results := make([]Result, len(tasks))

	// Get sourcefinder configuration
	sfConfig, err := cfg.GetSourcefinderConfig()
	if err != nil {
		for i := range tasks {
			results[i] = Result{
				Index:   tasks[i].InputIndex,
				Site:    tasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("failed to parse sourcefinder config: %w", err),
			}
		}
		return results
	}

	if cfg.Debug {
		log.Printf("[SourceFinder Config] URL: %s, Engines: %v, MaxResults: %d, Model: %s",
			sfConfig.URL, sfConfig.Engines, sfConfig.MaxResults, sfConfig.Model)
	}

	for i, task := range tasks {
		if cfg.Debug {
			log.Printf("[API Task %d/%d] Processing Input %d on %s...",
				i+1, len(tasks), task.InputIndex, task.Site)
		}

		// Create sourcefinder agent with config
		agent := sourcefinder.NewAgentWithConfig(
			sfConfig.URL,
			cfg.SourcefinderAPIKey,
			cfg.Timeout,
			cfg.Debug,
			sfConfig.Engines,
			sfConfig.MaxResults,
			sfConfig.Model,
		)

		// Process the fact check using structured input
		response, err := agent.ProcessFromStructuredInput(
			ctx,
			task.Input.Input.Title,
			task.Input.Input.Content,
			task.Input.Input.SourceURLs,
		)
		if err != nil {
			results[i] = Result{
				Index:   task.InputIndex,
				Site:    task.Site,
				Success: false,
				Error:   fmt.Errorf("API task failed: %w", err),
			}
			if cfg.Debug {
				log.Printf("[API Task %d] Failed: %v", i, err)
			}
			continue
		}

		results[i] = Result{
			Index:   task.InputIndex,
			Site:    task.Site,
			Success: true,
			Text:    response.Response.Text,
			JobID:   response.Metadata.JobID,
		}

		log.Printf("[API Task %d] Completed Input %d on %s",
			i+1, task.InputIndex, task.Site)
	}

	return results
}

// processBrowserTasks handles browser-based tasks (chatgpt, grok) using three-phase approach
func processBrowserTasks(cfg *config.Config, inputs []InputSource, template string, browserTasks []Task) []Result {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout+120*time.Second)
	defer cancel()

	// Initialize MCP client
	mcpCommand := cfg.BuildMCPCommand()
	if cfg.Debug {
		log.Printf("Connecting to MCP server: %s", mcpCommand)
	}
	mcpClient, err := mcp.NewClient(ctx, mcpCommand)
	if err != nil {
		// For browser-only errors, we can still return partial results if API tasks succeeded
		// But since this is browser tasks only, return error
		return []Result{
			{
				Success: false,
				Error:   fmt.Errorf("connecting to MCP server: %w", err),
			},
		}
	}
	defer mcpClient.Close()

	if cfg.Debug {
		log.Printf("MCP client connected")
	}

	cdt := mcp.NewChromeDevTools(mcpClient)

	// Create a results map for browser tasks
	browserResultsMap := make(map[int]Result) // key: task index in browserTasks

	// Phase 1: Open all tabs sequentially for each task
	log.Printf("Phase 1/3: Opening %d tabs...", len(browserTasks))
	if cfg.Debug {
		log.Printf("Phase 1: Opening %d tabs sequentially...", len(browserTasks))
	}

	for i := range browserTasks {
		if cfg.Debug {
			log.Printf("  [Task %d/%d] Opening tab for Input %d on %s...",
				i+1, len(browserTasks), browserTasks[i].InputIndex, browserTasks[i].Site)
		}

		// Get site URL
		siteURL := getSiteURL(browserTasks[i].Site)

		// Skip if empty URL (shouldn't happen for browser tasks)
		if siteURL == "" {
			browserResultsMap[i] = Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("empty site URL"),
			}
			if cfg.Debug {
				log.Printf("  [Task %d] Failed: empty site URL", i)
			}
			continue
		}

		// Open new page with target site
		_, err := cdt.NewPage(ctx, siteURL)
		if err != nil {
			browserResultsMap[i] = Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("failed to open new page: %w", err),
			}
			if cfg.Debug {
				log.Printf("  [Task %d] Failed to open tab: %v", i, err)
			}
			continue
		}

		// List pages to get the page ID of the newly created tab
		pages, err := cdt.ListPages(ctx)
		if err != nil {
			browserResultsMap[i] = Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("failed to list pages: %w", err),
			}
			if cfg.Debug {
				log.Printf("  [Task %d] Failed to list pages: %v", i, err)
			}
			continue
		}

		// The last page in the list is the one we just created
		if len(pages) == 0 {
			browserResultsMap[i] = Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("no pages found after opening new tab"),
			}
			continue
		}

		// Extract page ID from the last page
		lastPage := pages[len(pages)-1]
		pageID, ok := lastPage["pageId"].(float64)
		if !ok {
			browserResultsMap[i] = Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("failed to extract pageId from page info"),
			}
			continue
		}

		browserTasks[i].PageID = fmt.Sprintf("%.0f", pageID) // Convert float64 to string

		if cfg.Debug {
			log.Printf("  [Task %d] Tab opened with pageId: %s", i, browserTasks[i].PageID)
		}

		// Small delay between opening tabs
		time.Sleep(500 * time.Millisecond)
	}

	successfulTabs := 0
	for _, r := range browserResultsMap {
		if r.Error == nil {
			successfulTabs++
		}
	}
	log.Printf("Phase 1 complete: %d/%d tabs opened", successfulTabs, len(browserTasks))

	// Phase 2: Sequential input & submit (to avoid keyboard/click conflicts)
	log.Printf("Phase 2/3: Submitting inputs to %d tasks...", len(browserTasks))
	if cfg.Debug {
		log.Printf("Phase 2: Sequential input and submit for %d tasks...", len(browserTasks))
	}

	for i := range browserTasks {
		if _, exists := browserResultsMap[i]; exists && browserResultsMap[i].Error != nil {
			continue
		}

		if cfg.Debug {
			log.Printf("  [Task %d/%d] Submitting Input %d to %s (pageId: %s)...",
				i+1, len(browserTasks), browserTasks[i].InputIndex, browserTasks[i].Site, browserTasks[i].PageID)
		}

		// Select the page for this task
		if err := cdt.SelectPage(ctx, parseInt(browserTasks[i].PageID), false); err != nil {
			browserResultsMap[i] = Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("failed to select page: %w", err),
			}
			if cfg.Debug {
				log.Printf("  [Task %d] Failed to select page: %v", i, err)
			}
			continue
		}

		// Wait for page to stabilize after selection
		time.Sleep(500 * time.Millisecond)

		// Create isolated ChromeDevTools instance for this page
		cdtForPage := mcp.NewChromeDevTools(mcpClient)

		// Input and submit only (will wait in parallel phase)
		prompt := replaceTemplatePlaceholders(template, browserTasks[i].Input.Input)

		// Create the appropriate agent based on site
		if browserTasks[i].Site == "grok" {
			agent := grok.NewAgent(cdtForPage, cfg.Timeout, cfg.Debug)
			if err := agent.InputAndSubmitOnly(ctx, prompt); err != nil {
				browserResultsMap[i] = Result{
					Index:   browserTasks[i].InputIndex,
					Site:    browserTasks[i].Site,
					Success: false,
					Error:   fmt.Errorf("input and submit failed: %w", err),
				}
				if cfg.Debug {
					log.Printf("  [Task %d] Failed to submit: %v", i, err)
				}
				continue
			}
		} else {
			agent := chatgpt.NewAgent(cdtForPage, cfg.Timeout, cfg.Debug)
			if err := agent.InputAndSubmitOnly(ctx, prompt); err != nil {
				browserResultsMap[i] = Result{
					Index:   browserTasks[i].InputIndex,
					Site:    browserTasks[i].Site,
					Success: false,
					Error:   fmt.Errorf("input and submit failed: %w", err),
				}
				if cfg.Debug {
					log.Printf("  [Task %d] Failed to submit: %v", i, err)
				}
				continue
			}
		}

		if cfg.Debug {
			log.Printf("  [Task %d] Submitted successfully", i)
		}

		// Small delay between submissions
		time.Sleep(500 * time.Millisecond)
	}

	successfulSubmissions := 0
	for _, r := range browserResultsMap {
		if r.Error == nil {
			successfulSubmissions++
		}
	}
	log.Printf("Phase 2 complete: %d/%d inputs submitted", successfulSubmissions, len(browserTasks))

	// Phase 3: Sequential wait for responses
	// Important: We wait sequentially (not in parallel) because chrome-devtools-mcp
	// uses a single connection and all tool calls are serialized. When waiting in parallel,
	// SelectPage calls from different goroutines interfere with each other, causing
	// EvaluateScript to execute on the wrong page and fail to find elements.
	log.Printf("Phase 3/3: Waiting for %d responses...", len(browserTasks)-successfulSubmissions)
	if cfg.Debug {
		log.Printf("Phase 3: Sequential waiting for %d responses...", len(browserTasks))
	}

	completedCount := 0
	for i := range browserTasks {
		if _, exists := browserResultsMap[i]; exists && browserResultsMap[i].Error != nil {
			continue
		}

		if cfg.Debug {
			log.Printf("  [Task %d/%d] Waiting for Input %d on %s (pageId: %s)...",
				i+1, len(browserTasks), browserTasks[i].InputIndex, browserTasks[i].Site, browserTasks[i].PageID)
		}

		// Create isolated ChromeDevTools instance for this page
		cdtForPage := mcp.NewChromeDevTools(mcpClient)

		// Select the page for this task
		if err := cdtForPage.SelectPage(ctx, parseInt(browserTasks[i].PageID), false); err != nil {
			browserResultsMap[i] = Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("failed to select page: %w", err),
			}
			if cfg.Debug {
				log.Printf("  [Task %d] Failed to select page: %v", i, err)
			}
			continue
		}

		// Wait for response using the appropriate agent
		var responseText string

		if browserTasks[i].Site == "grok" {
			agent := grok.NewAgent(cdtForPage, cfg.Timeout, cfg.Debug)
			response, err := agent.WaitForResponse(ctx)
			if err != nil {
				browserResultsMap[i] = Result{
					Index:   browserTasks[i].InputIndex,
					Site:    browserTasks[i].Site,
					Success: false,
					Error:   fmt.Errorf("wait for response failed: %w", err),
				}
				if cfg.Debug {
					log.Printf("  [Task %d] Failed to wait for response: %v", i, err)
				}
				continue
			}
			responseText = response.Response.Text
		} else {
			agent := chatgpt.NewAgent(cdtForPage, cfg.Timeout, cfg.Debug)
			response, err := agent.WaitForResponse(ctx)
			if err != nil {
				browserResultsMap[i] = Result{
					Index:   browserTasks[i].InputIndex,
					Site:    browserTasks[i].Site,
					Success: false,
					Error:   fmt.Errorf("wait for response failed: %w", err),
				}
				if cfg.Debug {
					log.Printf("  [Task %d] Failed to wait for response: %v", i, err)
				}
				continue
			}
			responseText = response.Response.Text
		}

		browserResultsMap[i] = Result{
			Index:   browserTasks[i].InputIndex,
			Site:    browserTasks[i].Site,
			Success: true,
			Text:    responseText,
		}

		completedCount++
		log.Printf("Progress: [%d/%d] Completed Input %d on %s",
			completedCount, len(browserTasks)-successfulSubmissions, browserTasks[i].InputIndex, browserTasks[i].Site)

		if cfg.Debug {
			log.Printf("  [Task %d] Response received successfully for Input %d on %s",
				i, browserTasks[i].InputIndex, browserTasks[i].Site)
		}
	}

	// Convert browser results map to slice in order
	var browserResults []Result
	for i := 0; i < len(browserTasks); i++ {
		if result, ok := browserResultsMap[i]; ok {
			browserResults = append(browserResults, result)
		} else {
			// Missing result means it failed
			browserResults = append(browserResults, Result{
				Index:   browserTasks[i].InputIndex,
				Site:    browserTasks[i].Site,
				Success: false,
				Error:   fmt.Errorf("no result available"),
			})
		}
	}

	finalSuccessCount := 0
	for _, r := range browserResults {
		if r.Success {
			finalSuccessCount++
		}
	}
	log.Printf("Phase 3 complete: %d/%d responses received", finalSuccessCount, len(browserResults))

	return browserResults
}

// parseInt converts a string to int
func parseInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

// processInputNoNavigate processes a single input: input, submit, wait, extract (assumes page is already open)
func processInputNoNavigate(ctx context.Context, cfg *config.Config, cdt *mcp.ChromeDevTools, input InputSource, template string) Result {
	// Replace placeholders with actual input values
	prompt := replaceTemplatePlaceholders(template, input.Input)

	// Create agent and process the workflow (no navigation)
	agent := chatgpt.NewAgent(cdt, cfg.Timeout, cfg.Debug)

	// Input and submit only (page already navigated)
	if err := agent.InputAndSubmitOnly(ctx, prompt); err != nil {
		return Result{
			Index:   input.Index,
			Success: false,
			Error:   fmt.Errorf("input and submit failed: %w", err),
		}
	}

	// Wait for response
	response, err := agent.WaitForResponse(ctx)
	if err != nil {
		return Result{
			Index:   input.Index,
			Success: false,
			Error:   fmt.Errorf("wait for response failed: %w", err),
		}
	}

	return Result{
		Index:   input.Index,
		Success: true,
		Text:    response.Response.Text,
	}
}

// processInput processes a single input: navigate, input, submit, wait, extract
func processInput(ctx context.Context, cfg *config.Config, cdt *mcp.ChromeDevTools, input InputSource, template string) Result {
	// Replace placeholders with actual input values
	prompt := replaceTemplatePlaceholders(template, input.Input)

	// Create agent and process the full workflow
	agent := chatgpt.NewAgent(cdt, cfg.Timeout, cfg.Debug)

	// Navigate and submit (will open new tab via JavaScript if needed)
	if err := agent.NavigateAndSubmitWithNewTab(ctx, prompt); err != nil {
		return Result{
			Index:   input.Index,
			Success: false,
			Error:   fmt.Errorf("navigate and submit failed: %w", err),
		}
	}

	// Wait for response
	response, err := agent.WaitForResponse(ctx)
	if err != nil {
		return Result{
			Index:   input.Index,
			Success: false,
			Error:   fmt.Errorf("wait for response failed: %w", err),
		}
	}

	return Result{
		Index:   input.Index,
		Success: true,
		Text:    response.Response.Text,
	}
}

// inputAndSubmit navigates to ChatGPT and submits input (keyboard simulation, requires focus)
func inputAndSubmit(ctx context.Context, cfg *config.Config, cdt *mcp.ChromeDevTools, input InputSource, template string) error {
	// Replace placeholders with actual input values
	prompt := replaceTemplatePlaceholders(template, input.Input)

	// Create agent and process navigation + input + submit only
	agent := chatgpt.NewAgent(cdt, cfg.Timeout, cfg.Debug)
	return agent.NavigateAndSubmit(ctx, prompt)
}

// waitForResponse waits for ChatGPT response and extracts result (can be parallel)
func waitForResponse(ctx context.Context, cfg *config.Config, cdt *mcp.ChromeDevTools, input InputSource, _ string) Result {
	// Create agent and wait for response + extraction
	agent := chatgpt.NewAgent(cdt, cfg.Timeout, cfg.Debug)
	response, err := agent.WaitForResponse(ctx)
	if err != nil {
		return Result{
			Index:   input.Index,
			Success: false,
			Error:   err,
		}
	}

	return Result{
		Index:   input.Index,
		Success: true,
		Text:    response.Response.Text,
	}
}

// processSingleInput processes a single input source (legacy, for backward compatibility)
// Note: Tab should already be opened by processInputs Phase 1
func processSingleInput(ctx context.Context, cfg *config.Config, cdt *mcp.ChromeDevTools, input InputSource, template string) Result {
	// Replace placeholders with actual input values
	prompt := replaceTemplatePlaceholders(template, input.Input)

	// Create agent and process (will use the already-opened tab)
	agent := chatgpt.NewAgent(cdt, cfg.Timeout, cfg.Debug)
	response, err := agent.ProcessDocument(ctx, prompt)
	if err != nil {
		return Result{
			Index:   input.Index,
			Success: false,
			Error:   err,
		}
	}

	return Result{
		Index:   input.Index,
		Success: true,
		Text:    response.Response.Text,
	}
}

// errorResults creates error results for all inputs
func errorResults(inputs []InputSource, err error) []Result {
	results := make([]Result, len(inputs))
	for i, input := range inputs {
		results[i] = Result{
			Index:   input.Index,
			Site:    "",
			Success: false,
			Error:   err,
		}
	}
	return results
}

// errorResultsMultiSite creates error results for all input-site combinations
func errorResultsMultiSite(inputs []InputSource, sites []string, err error) []Result {
	var results []Result
	for _, input := range inputs {
		for _, site := range sites {
			results = append(results, Result{
				Index:   input.Index,
				Site:    site,
				Success: false,
				Error:   err,
			})
		}
	}
	return results
}

// formatJSONIfPossible tries to format the text as JSON, returns original if not JSON
func formatJSONIfPossible(text string) string {
	trimmed := strings.TrimSpace(text)

	// Check if it looks like JSON
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return text
	}

	// Try to parse as JSON
	var data interface{}
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		// Not valid JSON, return original
		return text
	}

	// Format with indentation
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		// Failed to format, return original
		return text
	}

	return string(formatted)
}

// sanitizeFilename sanitizes a string for use as a filename
func sanitizeFilename(name string, maxLength int) string {
	// Remove or replace invalid filename characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	sanitized := name
	for _, char := range invalidChars {
		sanitized = strings.ReplaceAll(sanitized, char, "_")
	}

	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)

	// Truncate if too long
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}

	// Remove trailing underscore if truncated
	sanitized = strings.TrimRight(sanitized, "_")

	return sanitized
}

// generateOutputFilename generates an output filename based on the base path and title
func generateOutputFilename(basePath string, title string, index int) string {
	// Extract base name and extension
	ext := filepath.Ext(basePath)
	baseName := strings.TrimSuffix(filepath.Base(basePath), ext)

	// Sanitize title for filename (max 50 chars)
	sanitizedTitle := sanitizeFilename(title, 50)

	// Generate filename
	if sanitizedTitle != "" {
		return fmt.Sprintf("%s_%s%s", baseName, sanitizedTitle, ext)
	}

	// Fallback to index if no title
	return fmt.Sprintf("%s_%d%s", baseName, index, ext)
}

// outputResults outputs all results
func outputResults(cfg *config.Config, inputs []InputSource, results []Result) {
	// Group results by input index and site
	resultMap := make(map[string]*Result)
	for i := range results {
		if results[i].Success {
			key := fmt.Sprintf("%d-%s", results[i].Index, results[i].Site)
			resultMap[key] = &results[i]
		} else if cfg.Debug {
			log.Printf("Input %d on %s error: %v", results[i].Index, results[i].Site, results[i].Error)
		}
	}

	// Create a map of inputs by index for easy access
	inputMap := make(map[int]*InputSource)
	for i := range inputs {
		inputMap[inputs[i].Index] = &inputs[i]
	}

	// Output results in order
	inputCount := 0
	for _, result := range results {
		if result.Index > inputCount {
			inputCount = result.Index
		}
	}

	// If output path is specified, write separate files for each input
	if cfg.OutputPath != "" {
		for inputIdx := 0; inputIdx <= inputCount; inputIdx++ {
			var outputs []string
			hasAnyResult := false

			// Get input information
			var inputTitle string
			if input, inputExists := inputMap[inputIdx]; inputExists {
				inputTitle = input.Input.Title
			}

			// Collect all results for this input
			for _, site := range cfg.Sites {
				key := fmt.Sprintf("%d-%s", inputIdx, site)
				if result, exists := resultMap[key]; exists {
					if !hasAnyResult {
						// Add input separator
						if inputCount > 0 {
							outputs = append(outputs, fmt.Sprintf("=== Input %d ===", inputIdx))
						}

						// Add structured input information if available
						if input, inputExists := inputMap[inputIdx]; inputExists {
							if input.Input.URL != "" {
								outputs = append(outputs, fmt.Sprintf("URL: %s", input.Input.URL))
							}
							if input.Input.Title != "" {
								outputs = append(outputs, fmt.Sprintf("Title: %s", input.Input.Title))
							}
							if input.Input.Content != "" {
								outputs = append(outputs, fmt.Sprintf("Content: %s", input.Input.Content))
							}
							if len(input.Input.SourceURLs) > 0 {
								outputs = append(outputs, fmt.Sprintf("Source URLs: %s", strings.Join(input.Input.SourceURLs, ", ")))
							}
						}

						hasAnyResult = true
					}

					// Add site label if multiple sites or if sourcefinder with job ID
					if len(cfg.Sites) > 1 {
						if result.Site == "sourcefinder" && result.JobID != "" {
							outputs = append(outputs, fmt.Sprintf("--- %s (job_id:%s) ---", result.Site, result.JobID))
						} else {
							outputs = append(outputs, fmt.Sprintf("--- %s ---", result.Site))
						}
					} else if result.Site == "sourcefinder" && result.JobID != "" {
						// Single site case - show job ID for sourcefinder
						outputs = append(outputs, fmt.Sprintf("--- %s (job_id:%s) ---", result.Site, result.JobID))
					}

					// Try to format as JSON if possible
					formattedText := formatJSONIfPossible(result.Text)
					outputs = append(outputs, formattedText)
				}
			}

			// Only write file if we have results
			if hasAnyResult {
				// Generate output filename
				outputFilename := generateOutputFilename(cfg.OutputPath, inputTitle, inputIdx)

				// Join outputs
				finalOutput := strings.Join(outputs, "\n\n")

				// Write to file
				if err := os.WriteFile(outputFilename, []byte(finalOutput), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing output file %s: %v\n", outputFilename, err)
					os.Exit(1)
				}

				if cfg.Debug {
					log.Printf("Output written to: %s", outputFilename)
				}
			}
		}

		if cfg.Debug {
			successCount := 0
			for _, r := range results {
				if r.Success {
					successCount++
				}
			}
			log.Printf("Done! Processed %d/%d inputs successfully", successCount, len(results))
		}

		return
	}

	// If no output path, print to stdout (original behavior)
	var outputs []string

	for inputIdx := 0; inputIdx <= inputCount; inputIdx++ {
		hasAnyResult := false
		for _, site := range cfg.Sites {
			key := fmt.Sprintf("%d-%s", inputIdx, site)
			if result, exists := resultMap[key]; exists {
				if !hasAnyResult {
					// Add input separator for multiple inputs
					if inputCount > 0 {
						if len(outputs) > 0 {
							outputs = append(outputs, "")
						}
						outputs = append(outputs, fmt.Sprintf("=== Input %d ===", inputIdx))
					}

					// Add structured input information if available
					if input, inputExists := inputMap[inputIdx]; inputExists {
						if input.Input.URL != "" {
							outputs = append(outputs, fmt.Sprintf("URL: %s", input.Input.URL))
						}
						if input.Input.Title != "" {
							outputs = append(outputs, fmt.Sprintf("Title: %s", input.Input.Title))
						}
						if input.Input.Content != "" {
							outputs = append(outputs, fmt.Sprintf("Content: %s", input.Input.Content))
						}
						if len(input.Input.SourceURLs) > 0 {
							outputs = append(outputs, fmt.Sprintf("Source URLs: %s", strings.Join(input.Input.SourceURLs, ", ")))
						}
					}

					hasAnyResult = true
				}

				// Add site label if multiple sites or if sourcefinder with job ID
				if len(cfg.Sites) > 1 {
					if result.Site == "sourcefinder" && result.JobID != "" {
						outputs = append(outputs, fmt.Sprintf("--- %s (job_id:%s) ---", result.Site, result.JobID))
					} else {
						outputs = append(outputs, fmt.Sprintf("--- %s ---", result.Site))
					}
				} else if result.Site == "sourcefinder" && result.JobID != "" {
					// Single site case - show job ID for sourcefinder
					outputs = append(outputs, fmt.Sprintf("--- %s (job_id:%s) ---", result.Site, result.JobID))
				}

				// Try to format as JSON if possible
				formattedText := formatJSONIfPossible(result.Text)
				outputs = append(outputs, formattedText)
			}
		}
	}

	// Join outputs
	finalOutput := strings.Join(outputs, "\n\n")

	// Print to stdout
	fmt.Println(finalOutput)

	if cfg.Debug {
		successCount := 0
		for _, r := range results {
			if r.Success {
				successCount++
			}
		}
		log.Printf("Done! Processed %d/%d inputs successfully", successCount, len(results))
	}
}
