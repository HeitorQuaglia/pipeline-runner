package pipeline

import (
	"time"

	"basic-container-runtime/pkg/workflow"
)

type PipelineStatus string

const (
	PipelineStatusDraft     PipelineStatus = "draft"
	PipelineStatusActive    PipelineStatus = "active"
	PipelineStatusInactive  PipelineStatus = "inactive"
	PipelineStatusArchived  PipelineStatus = "archived"
)

type TriggerType string

const (
	TriggerTypeManual     TriggerType = "manual"
	TriggerTypeSchedule   TriggerType = "schedule"
	TriggerTypeWebhook    TriggerType = "webhook"
	TriggerTypeEvent      TriggerType = "event"
	TriggerTypeFileChange TriggerType = "file_change"
)

type EventType string

const (
	EventTypePush         EventType = "push"
	EventTypePullRequest  EventType = "pull_request"
	EventTypeTag          EventType = "tag"
	EventTypeRelease      EventType = "release"
	EventTypeIssue        EventType = "issue"
	EventTypeArtifact     EventType = "artifact"
	EventTypeCustom       EventType = "custom"
)

type Pipeline struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string            `json:"version,omitempty" yaml:"version,omitempty"`
	
	Repository  *Repository       `json:"repository,omitempty" yaml:"repository,omitempty"`
	
	Triggers    []Trigger         `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Variables   map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	Secrets     []string          `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	
	Workflow    workflow.Workflow `json:"workflow" yaml:"workflow"`
	
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	
	Status      PipelineStatus    `json:"status"`
	
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	
	LastRunAt   *time.Time        `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time        `json:"next_run_at,omitempty"`
}

type Trigger struct {
	ID          string            `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
	Type        TriggerType       `json:"type" yaml:"type"`
	Enabled     bool              `json:"enabled" yaml:"enabled"`
	
	Schedule    *ScheduleTrigger  `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	Webhook     *WebhookTrigger   `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Event       *EventTrigger     `json:"event,omitempty" yaml:"event,omitempty"`
	FileChange  *FileChangeTrigger `json:"file_change,omitempty" yaml:"file_change,omitempty"`
	
	Conditions  []TriggerCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ScheduleTrigger struct {
	Cron        string            `json:"cron" yaml:"cron"`
	Timezone    string            `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	OnlyIfChanged bool            `json:"only_if_changed,omitempty" yaml:"only_if_changed,omitempty"`
}

type WebhookTrigger struct {
	URL         string            `json:"url,omitempty"`
	Secret      string            `json:"secret,omitempty" yaml:"secret,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Methods     []string          `json:"methods,omitempty" yaml:"methods,omitempty"`
}

type EventTrigger struct {
	Event       EventType         `json:"event" yaml:"event"`
	Source      string            `json:"source,omitempty" yaml:"source,omitempty"`
	Filter      map[string]string `json:"filter,omitempty" yaml:"filter,omitempty"`
}

type FileChangeTrigger struct {
	Paths       []string          `json:"paths" yaml:"paths"`
	IgnorePaths []string          `json:"ignore_paths,omitempty" yaml:"ignore_paths,omitempty"`
	Pattern     string            `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Recursive   bool              `json:"recursive,omitempty" yaml:"recursive,omitempty"`
}

type TriggerCondition struct {
	Field       string            `json:"field" yaml:"field"`
	Operator    string            `json:"operator" yaml:"operator"`
	Value       string            `json:"value" yaml:"value"`
	CaseSensitive bool            `json:"case_sensitive,omitempty" yaml:"case_sensitive,omitempty"`
}

type Repository struct {
	URL         string            `json:"url" yaml:"url"`
	Branch      string            `json:"branch,omitempty" yaml:"branch,omitempty"`
	Commit      string            `json:"commit,omitempty" yaml:"commit,omitempty"`
	Tag         string            `json:"tag,omitempty" yaml:"tag,omitempty"`
	
	Credentials *RepoCredentials  `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	
	SubPath     string            `json:"sub_path,omitempty" yaml:"sub_path,omitempty"`
	Shallow     bool              `json:"shallow,omitempty" yaml:"shallow,omitempty"`
	Depth       int               `json:"depth,omitempty" yaml:"depth,omitempty"`
}

type RepoCredentials struct {
	Type        string            `json:"type" yaml:"type"`
	Username    string            `json:"username,omitempty" yaml:"username,omitempty"`
	Password    string            `json:"password,omitempty" yaml:"password,omitempty"`
	Token       string            `json:"token,omitempty" yaml:"token,omitempty"`
	PrivateKey  string            `json:"private_key,omitempty" yaml:"private_key,omitempty"`
	Passphrase  string            `json:"passphrase,omitempty" yaml:"passphrase,omitempty"`
}

type PipelineRun struct {
	ID          string            `json:"id"`
	PipelineID  string            `json:"pipeline_id"`
	Number      int               `json:"number"`
	
	Status      workflow.WorkflowStatus `json:"status"`
	
	TriggerID   string            `json:"trigger_id,omitempty"`
	TriggerType TriggerType       `json:"trigger_type"`
	TriggedBy   string            `json:"triggered_by,omitempty"`
	
	Variables   map[string]string `json:"variables,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	
	Workflow    workflow.Workflow `json:"workflow"`
	
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	Duration    *time.Duration    `json:"duration,omitempty"`
	
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type PipelineTemplate struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Category    string            `json:"category,omitempty" yaml:"category,omitempty"`
	
	Template    Pipeline          `json:"template" yaml:"template"`
	Parameters  []TemplateParam   `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	
	UsageCount  int               `json:"usage_count"`
}

type TemplateParam struct {
	Name        string            `json:"name" yaml:"name"`
	Type        string            `json:"type" yaml:"type"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool              `json:"required,omitempty" yaml:"required,omitempty"`
	Default     interface{}       `json:"default,omitempty" yaml:"default,omitempty"`
	Options     []string          `json:"options,omitempty" yaml:"options,omitempty"`
	Validation  string            `json:"validation,omitempty" yaml:"validation,omitempty"`
}