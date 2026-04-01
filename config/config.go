package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// stringSlice is a custom flag type that allows multiple -text flags
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Config holds all configuration for the ChatGPT agent
type Config struct {
	// InputTexts is the list of texts to fact-check (from multiple -text flags)
	InputTexts []string

	// InputFile is the path to a file with multiple inputs (one per line)
	InputFile string

	// DocumentPath is the path to the input document file (deprecated, use InputFile)
	DocumentPath string

	// PromptTemplate is the path to the prompt template file
	PromptTemplate string

	// Timeout is how long to wait for ChatGPT to complete its response
	Timeout time.Duration

	// Debug enables verbose logging
	Debug bool

	// ScreenshotDir is where to save debug screenshots (empty = no screenshots)
	ScreenshotDir string

	// OutputPath is where to write the JSON output (empty = stdout)
	OutputPath string

	// MCPCommand is the command to spawn the chrome-devtools-mcp server
	MCPCommand string

	// BrowserURL is the Chrome DevTools Protocol endpoint to connect to
	// e.g., http://localhost:9222. If set, connects to existing Chrome instance.
	BrowserURL string

	// UserDataDir is the Chrome user data directory to use
	// e.g., /tmp/chrome-session. If set, Chrome will use this profile.
	UserDataDir string

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// Site is the target site to use (chatgpt, grok, sourcefinder, or all)
	Site string

	// Sites is the list of sites to use (parsed from Site)
	Sites []string

	// SourcefinderAPIKey is the API key for SourceFinder authentication
	SourcefinderAPIKey string

	// SourcefinderCommand is the command string to configure sourcefinder parameters
	// Format: "url=<url> --engines=<engines> --max-results=<n> --model=<model>"
	// Example: "url=https://api.example.com --engines=google,twitter --max-results=10 --model=gpt-5"
	SourcefinderCommand string

	// MaxConcurrentTabs is the maximum number of browser tabs to keep open simultaneously
	// Set to 1 for fully sequential execution (open one tab, process it, close it, then next)
	// Set to 5 for default parallelism
	MaxConcurrentTabs int
}

const (
	// DefaultTimeout is the default time to wait for ChatGPT response
	DefaultTimeout = 120 * time.Second

	// DefaultMCPCommand is the default command to spawn the MCP server
	DefaultMCPCommand = "npx -y chrome-devtools-mcp@latest"

	// DefaultMaxRetries is the default maximum retry attempts
	DefaultMaxRetries = 3

	// DefaultMaxConcurrentTabs is the default maximum number of tabs to open simultaneously
	DefaultMaxConcurrentTabs = 5

	// DefaultSourcefinderURL is the default SourceFinder API URL
	DefaultSourcefinderURL = "https://sourcefinder-api.coinpost.ai"

	// DefaultSourcefinderCommand is the default sourcefinder command
	DefaultSourcefinderCommand = "url=" + DefaultSourcefinderURL + " --engines=google,twitter --max-results=5 --model=gpt-5"
)

// Parse parses CLI flags and returns the configuration
func Parse() (*Config, error) {
	var textFlags stringSlice
	cfg := &Config{
		Timeout:           DefaultTimeout,
		MCPCommand:        DefaultMCPCommand,
		MaxRetries:        DefaultMaxRetries,
		MaxConcurrentTabs: DefaultMaxConcurrentTabs,
		PromptTemplate:    "user-prompt-input.md",
		InputTexts:        []string{},
		Site:              "chatgpt",
	}

	flag.Var(&textFlags, "text", "Input text to fact-check (can be specified multiple times)")
	flag.StringVar(&cfg.InputFile, "input-file", "", "File containing multiple inputs (one per line)")
	flag.StringVar(&cfg.PromptTemplate, "template", "user-prompt-input.md", "Path to prompt template file")
	flag.StringVar(&cfg.MCPCommand, "mcp-command", DefaultMCPCommand, "Command to spawn MCP server")
	flag.DurationVar(&cfg.Timeout, "timeout", DefaultTimeout, "Time to wait for response (e.g., 30s, 2m)")
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable debug logging")
	flag.StringVar(&cfg.ScreenshotDir, "screenshot-dir", "", "Directory to save debug screenshots")
	flag.StringVar(&cfg.OutputPath, "output", "", "JSON output file (default: stdout)")
	flag.StringVar(&cfg.BrowserURL, "browser-url", "", "Chrome DevTools Protocol endpoint (e.g., http://localhost:9222)")
	flag.StringVar(&cfg.Site, "site", "chatgpt", "Target site: chatgpt, grok, sourcefinder, all, or comma-separated list")
	flag.StringVar(&cfg.SourcefinderCommand, "sourcefinder-command", DefaultSourcefinderCommand, "SourceFinder command parameters (url, engines, max-results, model)")
	flag.StringVar(&cfg.SourcefinderAPIKey, "sourcefinder-api-key", "", "SourceFinder API key (if authentication enabled)")
	flag.IntVar(&cfg.MaxConcurrentTabs, "max-concurrent-tabs", DefaultMaxConcurrentTabs, "Maximum number of tabs to keep open simultaneously (1=sequential, 5=default)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nExamples:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  # Read inputs from file (one per line)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s -input-file inputs.txt\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  # Multiple -text flags (each opens a new tab)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s -text \"Claim 1\" -text \"Claim 2\" -text \"Claim 3\"\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  # Single input from file (legacy)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s inputs.txt\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  # Use custom template\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s -text \"...\" -template my-template.md\n", os.Args[0])
	}

	flag.Parse()

	cfg.InputTexts = textFlags

	// Handle legacy positional argument
	if len(textFlags) == 0 && cfg.InputFile == "" {
		args := flag.Args()
		if len(args) > 0 {
			// Check if it's a file or direct text
			if _, err := os.Stat(args[0]); err == nil {
				cfg.InputFile = args[0]
			} else {
				cfg.InputTexts = []string{args[0]}
			}
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Parse sourcefinder command if provided
	if err := cfg.ParseSourcefinderCommand(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Either InputTexts, InputFile, or DocumentPath must be provided
	if len(c.InputTexts) == 0 && c.InputFile == "" && c.DocumentPath == "" {
		return fmt.Errorf("input required: specify -text (one or more), -input-file, or a positional argument")
	}

	if c.Timeout < 1*time.Second {
		return fmt.Errorf("timeout must be at least 1 second")
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	if c.MaxConcurrentTabs < 1 {
		return fmt.Errorf("max-concurrent-tabs must be at least 1")
	}

	// Parse and validate site option
	if c.Site == "all" {
		c.Sites = []string{"chatgpt", "grok", "sourcefinder"}
	} else if strings.Contains(c.Site, ",") {
		// Parse comma-separated list
		sites := strings.Split(c.Site, ",")
		for i, site := range sites {
			sites[i] = strings.TrimSpace(site)
			if sites[i] != "chatgpt" && sites[i] != "grok" && sites[i] != "sourcefinder" {
				return fmt.Errorf("invalid site in list: %s (must be 'chatgpt', 'grok', or 'sourcefinder')", sites[i])
			}
		}
		c.Sites = sites
	} else if c.Site == "chatgpt" || c.Site == "grok" || c.Site == "sourcefinder" {
		c.Sites = []string{c.Site}
	} else {
		return fmt.Errorf("site must be 'chatgpt', 'grok', 'sourcefinder', 'all', or comma-separated list, got: %s", c.Site)
	}

	// Validate API key for sourcefinder
	for _, site := range c.Sites {
		if site == "sourcefinder" && c.SourcefinderAPIKey == "" {
			return fmt.Errorf("API key required for sourcefinder (use -sourcefinder-api-key)")
		}
	}

	return nil
}

// BuildMCPCommand builds the final MCP command with all necessary arguments
func (c *Config) BuildMCPCommand() string {
	cmd := c.MCPCommand

	// Add browser-url if specified
	if c.BrowserURL != "" {
		cmd += " --browser-url=" + c.BrowserURL
	}

	return cmd
}

// SourcefinderConfig holds the parsed sourcefinder command parameters
type SourcefinderConfig struct {
	URL        string
	Engines    []string
	MaxResults int
	Model      string
}

// ParseSourcefinderCommand parses the sourcefinder command string
// Format: "url=<url> --engines=<engines> --max-results=<n> --model=<model>"
// Example: "url=https://api.example.com --engines=google,twitter --max-results=10 --model=gpt-5"
func (c *Config) ParseSourcefinderCommand() error {
	// Parse the command string
	_, err := parseSourcefinderCommandString(c.SourcefinderCommand)
	if err != nil {
		return fmt.Errorf("failed to parse sourcefinder command: %w", err)
	}

	return nil
}

// parseSourcefinderCommandString parses a sourcefinder command string
func parseSourcefinderCommandString(cmdStr string) (*SourcefinderConfig, error) {
	config := &SourcefinderConfig{
		URL:        DefaultSourcefinderURL,
		Engines:    []string{"google", "twitter"},
		MaxResults: 5,
		Model:      "gpt-5",
	}

	if cmdStr == "" {
		return config, nil
	}

	// Parse key=value and --key=value patterns
	parts := strings.Fields(cmdStr)
	for _, part := range parts {
		// Handle --key=value format
		if strings.HasPrefix(part, "--") {
			part = strings.TrimPrefix(part, "--")
		}

		// Split on =
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "url":
			config.URL = value
		case "engines":
			// Parse comma-separated engines
			engines := strings.Split(value, ",")
			for i, e := range engines {
				engines[i] = strings.TrimSpace(e)
			}
			config.Engines = engines
		case "max-results":
			maxResults, err := fmt.Sscanf(value, "%d", &config.MaxResults)
			if err != nil || maxResults != 1 {
				return nil, fmt.Errorf("invalid max-results value: %s", value)
			}
		case "model":
			config.Model = value
		}
	}

	return config, nil
}

// GetSourcefinderConfig returns the parsed sourcefinder configuration
func (c *Config) GetSourcefinderConfig() (*SourcefinderConfig, error) {
	return parseSourcefinderCommandString(c.SourcefinderCommand)
}
