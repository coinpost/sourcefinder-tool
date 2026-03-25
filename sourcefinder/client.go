package sourcefinder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Client represents the sourcefinder API client
type Client struct {
	config     *Config
	httpClient *http.Client
}

// NewClient creates a new sourcefinder API client
func NewClient(config *Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SubmitJob submits a new fact-checking job
func (c *Client) SubmitJob(ctx context.Context, req *JobRequest) (*JobSubmitResponse, error) {
	if c.config.Debug {
		log.Printf("[SourceFinder] Submitting job: title='%s', content_length=%d",
			req.Title, len(req.Content))
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if c.config.Debug {
		log.Printf("[SourceFinder] Request body: %s", string(jsonData))
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/sources", c.config.BaseURL)
	if c.config.Debug {
		log.Printf("[SourceFinder] POST %s", url)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if c.config.Debug {
		log.Printf("[SourceFinder] Response status: %d %s", resp.StatusCode, resp.Status)
		log.Printf("[SourceFinder] Response headers: %v", resp.Header)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if c.config.Debug {
		log.Printf("[SourceFinder] Response body (%d bytes): %s", len(body), string(body))
	}

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - try wrapped format first, then direct format
	var wrappedResp JobSubmitDataWrapper
	var submitResp JobSubmitResponse

	// Try to parse as wrapped format {"data": {"job_id": "..."}}
	if err := json.Unmarshal(body, &wrappedResp); err == nil && wrappedResp.Data.JobID != "" {
		submitResp = wrappedResp.Data
	} else {
		// Try to parse as direct format {"job_id": "..."}
		if err := json.Unmarshal(body, &submitResp); err != nil {
			if c.config.Debug {
				log.Printf("[SourceFinder] Failed to parse submit response: %v", err)
				log.Printf("[SourceFinder] Raw response body: %s", string(body))
			}
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
	}

	if c.config.Debug {
		log.Printf("[SourceFinder] Job submitted successfully: job_id=%s", submitResp.JobID)
		if submitResp.JobID == "" {
			log.Printf("[SourceFinder] WARNING: job_id is empty!")
			log.Printf("[SourceFinder] Raw response: %s", string(body))
		}
	}

	return &submitResp, nil
}

// GetJob retrieves the status and result of a job
func (c *Client) GetJob(ctx context.Context, jobID string) (*JobResponse, error) {
	if c.config.Debug {
		log.Printf("[SourceFinder] Getting job status: job_id=%s", jobID)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/sources/%s", c.config.BaseURL, jobID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - try wrapped format first, then direct format
	var wrappedResp JobResponseDataWrapper
	var jobResp JobResponse

	// Try to parse as wrapped format {"data": {...}}
	if err := json.Unmarshal(body, &wrappedResp); err == nil && wrappedResp.Data.ID != "" {
		jobResp = wrappedResp.Data
	} else {
		// Try to parse as direct format {...}
		if err := json.Unmarshal(body, &jobResp); err != nil {
			if c.config.Debug {
				log.Printf("[SourceFinder] Failed to parse job response: %v", err)
				log.Printf("[SourceFinder] Raw response body: %s", string(body))
			}
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
	}

	if c.config.Debug {
		log.Printf("[SourceFinder] Job status: job_id=%s, status=%s", jobID, jobResp.Status)
	}

	return &jobResp, nil
}

// WaitForCompletion waits for a job to complete and returns the result
func (c *Client) WaitForCompletion(ctx context.Context, jobID string) (*JobResponse, error) {
	if c.config.Debug {
		log.Printf("[SourceFinder] Waiting for job completion: job_id=%s", jobID)
	}

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Poll interval
	pollInterval := 3 * time.Second
	attempts := 0

	for {
		attempts++

		// Check if context is cancelled
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for job completion after %d attempts", attempts)
		default:
		}

		// Get job status
		jobResp, err := c.GetJob(timeoutCtx, jobID)
		if err != nil {
			if c.config.Debug {
				log.Printf("[SourceFinder] Attempt %d: Error getting job status: %v", attempts, err)
			}
			// Wait before retry
			time.Sleep(pollInterval)
			continue
		}

		// Check job status
		switch jobResp.Status {
		case JobStatusCompleted:
			if c.config.Debug {
				log.Printf("[SourceFinder] Job completed after %d attempts", attempts)
			}
			return jobResp, nil

		case JobStatusFailed:
			return nil, fmt.Errorf("job failed: %s", jobResp.Error)

		case JobStatusPending, JobStatusProcessing, JobStatusRunning:
			if c.config.Debug {
				log.Printf("[SourceFinder] Attempt %d: Job still running (status=%s)", attempts, jobResp.Status)
			}
			// Wait before next poll
			time.Sleep(pollInterval)

		default:
			return nil, fmt.Errorf("unknown job status: %s", jobResp.Status)
		}
	}
}

// ProcessFactCheck submits a job and waits for completion
func (c *Client) ProcessFactCheck(ctx context.Context, title, content string, sourceURLs []string) (*JobResponse, error) {
	startTime := time.Now()

	// Use config parameters with defaults
	engines := c.config.Engines
	if len(engines) == 0 {
		engines = []string{"google", "tavily"}
	}

	maxResults := c.config.MaxResults
	if maxResults == 0 {
		maxResults = 5
	}

	model := c.config.Model
	if model == "" {
		model = "gpt-5"
	}

	// Create job request
	req := &JobRequest{
		Title:      title,
		Content:    content,
		SourceURLs: sourceURLs,
		Engines:    engines,
		MaxResults: maxResults,
		Model:      model,
	}

	if c.config.Debug {
		log.Printf("[SourceFinder] Creating job request with engines=%v, max_results=%d, model=%s",
			engines, maxResults, model)
	}

	// Submit job
	submitResp, err := c.SubmitJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	// Wait for completion
	jobResp, err := c.WaitForCompletion(ctx, submitResp.JobID)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for completion: %w", err)
	}

	duration := time.Since(startTime)
	if c.config.Debug {
		log.Printf("[SourceFinder] Fact check completed in %v", duration)
	}

	return jobResp, nil
}

// FormatAsJSON formats the job result as JSON string matching the expected format
func FormatAsJSON(jobResp *JobResponse) (string, error) {
	if jobResp.Status != JobStatusCompleted {
		return "", fmt.Errorf("job is not completed: %s", jobResp.Status)
	}

	// Build output in the format expected by the template
	output := map[string]interface{}{
		"result":           jobResp.Result.TruthAssessment.TruthProbability > 0.5,
		"original_sources": make([]map[string]interface{}, 0),
	}

	for _, source := range jobResp.Result.PrimarySources {
		sourceMap := map[string]interface{}{
			"name":         source.SourceType,
			"title":        source.Title,
			"publish_date": source.PublishedAt,
			"source_url":   source.URL,
		}
		output["original_sources"] = append(output["original_sources"].([]map[string]interface{}), sourceMap)
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(jsonData), nil
}
