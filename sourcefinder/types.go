package sourcefinder

import "time"

// JobRequest represents a request to submit a new fact-checking job
type JobRequest struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	SourceURLs []string `json:"source_urls,omitempty"`
	Engines    []string `json:"engines,omitempty"`
	MaxResults int      `json:"max_results,omitempty"`
	Model      string   `json:"model,omitempty"`
}

// JobSubmitResponse represents the response when submitting a job
type JobSubmitResponse struct {
	JobID string `json:"job_id"`
}

// JobSubmitDataWrapper represents the actual API response wrapper
type JobSubmitDataWrapper struct {
	Data JobSubmitResponse `json:"data"`
}

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusRunning    JobStatus = "running"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// PrimarySource represents a verified primary source
type PrimarySource struct {
	URL             string   `json:"url"`
	Title           string   `json:"title"`
	AuthorityScore  float64  `json:"authority_score"`
	RelevanceScore  float64  `json:"relevance_score"`
	SourceType      string   `json:"source_type"`
	Summary         string   `json:"summary"`
	PublishedAt     string   `json:"published_at"`
	Validated       *bool    `json:"validated"`
	IsNewsLike      bool     `json:"is_news_like"`
}

// TruthAssessment represents the truthfulness assessment
type TruthAssessment struct {
	TruthProbability     float64  `json:"truth_probability"`
	FalseProbability     float64  `json:"false_probability"`
	Assessment           string   `json:"assessment"`
	ConfidenceLevel      string   `json:"confidence_level"`
	Reasoning            string   `json:"reasoning"`
}

// SearchMetrics represents search performance metrics
type SearchMetrics struct {
	SearchRounds        int `json:"search_rounds"`
	AnalysisRounds      int `json:"analysis_rounds"`
	TotalSources        int `json:"total_sources"`
	QualitySourcesFound int `json:"quality_sources_found"`
}

// JobResult represents the result of a completed job
type JobResult struct {
	Statement       string           `json:"statement"`
	PrimarySources  []PrimarySource  `json:"primary_sources"`
	TruthAssessment TruthAssessment  `json:"truth_assessment"`
	SearchMetrics   SearchMetrics    `json:"search_metrics"`
	ProcessingTime  int              `json:"processing_time_ms"`
}

// JobResponse represents the response when getting job results
type JobResponse struct {
	ID     string    `json:"id"`
	Status JobStatus `json:"status"`
	Result JobResult `json:"result,omitempty"`
	Error  string    `json:"error,omitempty"`
}

// JobResponseDataWrapper represents the actual API response wrapper for job results
type JobResponseDataWrapper struct {
	Data JobResponse `json:"data"`
}

// Response represents the final response format matching other agents
type Response struct {
	Success  bool      `json:"success"`
	Response SourceMsg `json:"response"`
	Error    string    `json:"error,omitempty"`
	Metadata Metadata  `json:"metadata,omitempty"`
}

// SourceMsg represents the source finder message
type SourceMsg struct {
	Text string `json:"text"`
}

// Metadata represents metadata about the response
type Metadata struct {
	Duration  time.Duration `json:"duration"`
	ToolCalls int           `json:"tool_calls"`
	JobID     string        `json:"job_id,omitempty"` // SourceFinder job ID
}

// Config holds configuration for the sourcefinder client
type Config struct {
	BaseURL    string
	APIKey     string
	Timeout    time.Duration
	MaxRetries int
	Debug      bool
	Engines    []string
	MaxResults int
	Model      string
}
