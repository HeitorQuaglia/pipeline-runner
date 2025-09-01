package server

import (
	"time"
	
	"basic-container-runtime/pkg/workflow"
)

// WorkflowSubmission represents a workflow submitted via API
type WorkflowSubmission struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	YAML        string            `json:"yaml"`
	Variables   map[string]string `json:"variables,omitempty"`
	Secrets     map[string]string `json:"secrets,omitempty"`
}

// Job represents a job in the server queue
type Job struct {
	ID           string                 `json:"id"`
	WorkflowID   string                 `json:"workflow_id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	YAML         string                 `json:"yaml"`
	Variables    map[string]string      `json:"variables,omitempty"`
	Secrets      map[string]string      `json:"secrets,omitempty"`
	Status       JobStatus              `json:"status"`
	RunnerID     string                 `json:"runner_id,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Logs         []LogEntry             `json:"logs,omitempty"`
	Result       *JobResult             `json:"result,omitempty"`
	Priority     int                    `json:"priority"`
	Tags         []string               `json:"tags,omitempty"`
}

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running" 
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// JobResult represents the result of a completed job
type JobResult struct {
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	ExitCode     int                    `json:"exit_code"`
	Duration     time.Duration          `json:"duration"`
	Workflow     *workflow.Workflow     `json:"workflow,omitempty"`
	Statistics   *ExecutionStatistics   `json:"statistics,omitempty"`
}

// ExecutionStatistics contains job execution statistics
type ExecutionStatistics struct {
	JobsTotal     int           `json:"jobs_total"`
	JobsCompleted int           `json:"jobs_completed"`
	JobsFailed    int           `json:"jobs_failed"`
	TotalDuration time.Duration `json:"total_duration"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
}

// LogEntry represents a log entry from job execution
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"` // runner_id, job_name, etc.
}

// Runner represents a registered runner
type Runner struct {
	ID           string            `json:"id"`
	Name         string            `json:"name,omitempty"`
	Version      string            `json:"version,omitempty"`
	Status       RunnerStatus      `json:"status"`
	LastHeartbeat time.Time        `json:"last_heartbeat"`
	RegisteredAt  time.Time        `json:"registered_at"`
	CurrentJobID  string           `json:"current_job_id,omitempty"`
	Capabilities  []string         `json:"capabilities,omitempty"`
	Tags          []string         `json:"tags,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Address       string           `json:"address,omitempty"`
}

// RunnerStatus represents the status of a runner
type RunnerStatus string

const (
	RunnerStatusOnline   RunnerStatus = "online"
	RunnerStatusOffline  RunnerStatus = "offline"
	RunnerStatusBusy     RunnerStatus = "busy"
	RunnerStatusDisabled RunnerStatus = "disabled"
)

// RunnerRegistration represents a runner registration request
type RunnerRegistration struct {
	ID           string            `json:"id"`
	Name         string            `json:"name,omitempty"`
	Version      string            `json:"version,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// HeartbeatRequest represents a heartbeat from a runner
type HeartbeatRequest struct {
	RunnerID     string       `json:"runner_id"`
	Status       RunnerStatus `json:"status"`
	CurrentJobID string       `json:"current_job_id,omitempty"`
}

// JobAssignment represents a job assigned to a runner
type JobAssignment struct {
	Job        *Job              `json:"job"`
	ServerURL  string            `json:"server_url"`
	AuthToken  string            `json:"auth_token,omitempty"`
}

// LogSubmission represents logs submitted by a runner
type LogSubmission struct {
	JobID     string     `json:"job_id"`
	RunnerID  string     `json:"runner_id"`
	Logs      []LogEntry `json:"logs"`
}

// JobCompletionRequest represents a job completion report from runner
type JobCompletionRequest struct {
	JobID    string     `json:"job_id"`
	RunnerID string     `json:"runner_id"`
	Result   *JobResult `json:"result"`
}

// ServerStatus represents overall server status
type ServerStatus struct {
	Version         string    `json:"version"`
	StartTime       time.Time `json:"start_time"`
	JobsTotal       int       `json:"jobs_total"`
	JobsPending     int       `json:"jobs_pending"`
	JobsRunning     int       `json:"jobs_running"`
	JobsCompleted   int       `json:"jobs_completed"`
	JobsFailed      int       `json:"jobs_failed"`
	RunnersOnline   int       `json:"runners_online"`
	RunnersOffline  int       `json:"runners_offline"`
	RunnersBusy     int       `json:"runners_busy"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// JobListResponse represents paginated job list response
type JobListResponse struct {
	Jobs       []*Job            `json:"jobs"`
	Pagination *PaginationParams `json:"pagination"`
}

// RunnerListResponse represents paginated runner list response  
type RunnerListResponse struct {
	Runners    []*Runner         `json:"runners"`
	Pagination *PaginationParams `json:"pagination"`
}