package workflow

import (
	"time"
)

type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusCanceled  WorkflowStatus = "canceled"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusSkipped   JobStatus = "skipped"
	JobStatusCanceled  JobStatus = "canceled"
)

type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusRunning   CommandStatus = "running"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusFailed    CommandStatus = "failed"
)

type RestartPolicy string

const (
	RestartPolicyNever     RestartPolicy = "never"
	RestartPolicyOnFailure RestartPolicy = "on-failure"
	RestartPolicyAlways    RestartPolicy = "always"
)

type RuleOperator string

const (
	RuleOperatorEquals       RuleOperator = "equals"
	RuleOperatorNotEquals    RuleOperator = "not_equals"
	RuleOperatorContains     RuleOperator = "contains"
	RuleOperatorNotContains  RuleOperator = "not_contains"
	RuleOperatorStartsWith   RuleOperator = "starts_with"
	RuleOperatorEndsWith     RuleOperator = "ends_with"
	RuleOperatorRegex        RuleOperator = "regex"
	RuleOperatorExists       RuleOperator = "exists"
	RuleOperatorNotExists    RuleOperator = "not_exists"
)

type Workflow struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string            `json:"version,omitempty" yaml:"version,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	
	Variables   map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	Secrets     []string          `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Volumes     map[string]VolumeSpec `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	
	Jobs        []Job             `json:"jobs" yaml:"jobs"`
	Rules       []Rule            `json:"rules,omitempty" yaml:"rules,omitempty"`
	
	Timeout     *time.Duration    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty" yaml:"retry_policy,omitempty"`
	
	Status      WorkflowStatus    `json:"status"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type Job struct {
	ID           string            `json:"id" yaml:"id"`
	Name         string            `json:"name" yaml:"name"`
	Description  string            `json:"description,omitempty" yaml:"description,omitempty"`
	
	DependsOn    []string          `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Commands     []Command         `json:"commands" yaml:"commands"`
	Rules        []Rule            `json:"rules,omitempty" yaml:"rules,omitempty"`
	
	Environment  map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Variables    map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	
	Container    *ContainerSpec    `json:"container,omitempty" yaml:"container,omitempty"`
	Containers   []ContainerSpec   `json:"containers,omitempty" yaml:"containers,omitempty"`
	Resources    *ResourceSpec     `json:"resources,omitempty" yaml:"resources,omitempty"`
	
	Timeout      *time.Duration    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	RetryPolicy  *RetryPolicy      `json:"retry_policy,omitempty" yaml:"retry_policy,omitempty"`
	
	AllowFailure bool              `json:"allow_failure,omitempty" yaml:"allow_failure,omitempty"`
	RunnerTags   []string          `json:"runner_tags,omitempty" yaml:"runner_tags,omitempty"`
	
	When         Condition         `json:"when,omitempty" yaml:"when,omitempty"`
	
	Status       JobStatus         `json:"status"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	RunnerID     string            `json:"runner_id,omitempty"`
	
	Artifacts     []ArtifactSpec  `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	UsesArtifacts []ArtifactMount `json:"uses_artifacts,omitempty" yaml:"uses_artifacts,omitempty"`
}

type Command struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
	Script      string            `json:"script" yaml:"script"`
	Shell       string            `json:"shell,omitempty" yaml:"shell,omitempty"`
	
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	
	Timeout     *time.Duration    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty" yaml:"retry_policy,omitempty"`
	
	AllowFailure bool             `json:"allow_failure,omitempty" yaml:"allow_failure,omitempty"`
	ContinueOnError bool          `json:"continue_on_error,omitempty" yaml:"continue_on_error,omitempty"`
	
	Status      CommandStatus     `json:"status"`
	ExitCode    *int              `json:"exit_code,omitempty"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	
	Output      string            `json:"output,omitempty"`
	ErrorOutput string            `json:"error_output,omitempty"`
}

type Rule struct {
	ID          string       `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string       `json:"name,omitempty" yaml:"name,omitempty"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	
	Field       string       `json:"field" yaml:"field"`
	Operator    RuleOperator `json:"operator" yaml:"operator"`
	Value       string       `json:"value" yaml:"value"`
	
	CaseSensitive bool       `json:"case_sensitive,omitempty" yaml:"case_sensitive,omitempty"`
	Required      bool       `json:"required,omitempty" yaml:"required,omitempty"`
}

type ContainerSpec struct {
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
	Image       string            `json:"image" yaml:"image"`
	Tag         string            `json:"tag,omitempty" yaml:"tag,omitempty"`
	
	Entrypoint  []string          `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Command     []string          `json:"command,omitempty" yaml:"command,omitempty"`
	
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	
	Volumes     []VolumeMount     `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Ports       []PortMapping     `json:"ports,omitempty" yaml:"ports,omitempty"`
	
	Privileged  bool              `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	NetworkMode string            `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	
	RestartPolicy RestartPolicy   `json:"restart_policy,omitempty" yaml:"restart_policy,omitempty"`
	
	PullPolicy  string            `json:"pull_policy,omitempty" yaml:"pull_policy,omitempty"`
}

type VolumeMount struct {
	Name        string `json:"name" yaml:"name"`
	Source      string `json:"source,omitempty" yaml:"source,omitempty"`
	Destination string `json:"destination,omitempty" yaml:"destination,omitempty"`
	MountPath   string `json:"mount_path,omitempty" yaml:"mount_path,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty" yaml:"read_only,omitempty"`
}

type VolumeSpec struct {
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Type        string `json:"type" yaml:"type"`
	HostPath    string `json:"host_path,omitempty" yaml:"host_path,omitempty"`
	Size        string `json:"size,omitempty" yaml:"size,omitempty"`
}

type PortMapping struct {
	HostPort      int    `json:"host_port" yaml:"host_port"`
	ContainerPort int    `json:"container_port" yaml:"container_port"`
	Protocol      string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}

type ResourceSpec struct {
	CPU    ResourceLimit `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory ResourceLimit `json:"memory,omitempty" yaml:"memory,omitempty"`
	Disk   ResourceLimit `json:"disk,omitempty" yaml:"disk,omitempty"`
}

type ResourceLimit struct {
	Request string `json:"request,omitempty" yaml:"request,omitempty"`
	Limit   string `json:"limit,omitempty" yaml:"limit,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	BackoffStrategy string    `json:"backoff_strategy,omitempty" yaml:"backoff_strategy,omitempty"`
	InitialDelay    *time.Duration `json:"initial_delay,omitempty" yaml:"initial_delay,omitempty"`
	MaxDelay        *time.Duration `json:"max_delay,omitempty" yaml:"max_delay,omitempty"`
	Multiplier      float64   `json:"multiplier,omitempty" yaml:"multiplier,omitempty"`
}

type ArtifactSpec struct {
	Name        string   `json:"name" yaml:"name"`
	Path        string   `json:"path" yaml:"path"`
	Type        string   `json:"type,omitempty" yaml:"type,omitempty"`
	Patterns    []string `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	ExpireAfter *time.Duration `json:"expire_after,omitempty" yaml:"expire_after,omitempty"`
	Public      bool     `json:"public,omitempty" yaml:"public,omitempty"`
}

type ArtifactMount struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path" yaml:"path"`
}