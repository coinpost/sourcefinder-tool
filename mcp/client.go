package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Client wraps the MCP client functionality
type Client struct {
	client *mcpclient.Client
	cmd    string
	args   []string
}

// NewClient creates a new MCP client that will spawn the chrome-devtools-mcp server
func NewClient(ctx context.Context, mcpCommand string) (*Client, error) {
	// Parse the command string (e.g., "npx -y chrome-devtools-mcp@latest")
	parts := strings.Fields(mcpCommand)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty MCP command")
	}

	cmd := parts[0]
	args := parts[1:]

	c, err := mcpclient.NewStdioMCPClient(cmd, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Initialize the connection
	initTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "page-crawler",
				Version: "1.0.0",
			},
		},
	}

	_, err = c.Initialize(initTimeout, initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return &Client{
		client: c,
		cmd:    cmd,
		args:   args,
	}, nil
}

// CallTool calls a tool on the MCP server
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	}

	result, err := c.client.CallTool(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tool call failed (name=%s): %w", name, err)
	}

	// Check for tool-level errors
	if result.IsError {
		// Extract error details from result content
		errorMsg := extractErrorFromContent(result.Content)
		if errorMsg != "" {
			return result, fmt.Errorf("tool returned error (name=%s): %s", name, errorMsg)
		}
		return result, fmt.Errorf("tool returned error (name=%s)", name)
	}

	return result, nil
}

// extractErrorFromContent extracts error message from MCP content
func extractErrorFromContent(content []mcp.Content) string {
	if len(content) == 0 {
		return ""
	}

	// Try to extract text from different content types
	for _, item := range content {
		switch c := item.(type) {
		case mcp.TextContent:
			return c.Text
		case mcp.EmbeddedResource:
			switch rc := c.Resource.(type) {
			case mcp.TextResourceContents:
				return rc.Text
			}
		}
	}

	return ""
}

// Close closes the MCP client connection
func (c *Client) Close() error {
	// The stdio client manages the subprocess lifecycle
	// When we return, the subprocess will be cleaned up
	return nil
}
