package templates

// Template represents a reusable workflow template
type Template struct {
	Kind       string                    `yaml:"kind"`
	Version    int                       `yaml:"version"`
	Parameters []TemplateParameter       `yaml:"parameters,omitempty"`
	Steps      map[string]TemplateStep   `yaml:"steps"`
}

// TemplateParameter defines a parameter for the template
type TemplateParameter struct {
	Name        string `yaml:"name"`
	Default     string `yaml:"default,omitempty"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

// TemplateStep represents a step in a template (similar to SimpleStepSpec but with template support)
type TemplateStep struct {
	Summary       string                        `yaml:"summary,omitempty"`
	Image         string                        `yaml:"image"`
	Command       []string                      `yaml:"command,omitempty"`
	Containers    []TemplateContainerSpec       `yaml:"containers,omitempty"`
	DependsOn     []string                      `yaml:"depends_on,omitempty"`
	Env           map[string]string             `yaml:"env,omitempty"`
	Variables     map[string]string             `yaml:"variables,omitempty"`
	WorkDir       string                        `yaml:"workdir,omitempty"`
	When          interface{}                   `yaml:"when,omitempty"`
	Artifacts     []TemplateArtifactSpec        `yaml:"artifacts,omitempty"`
	UsesArtifacts []TemplateArtifactMount       `yaml:"uses_artifacts,omitempty"`
	
	// Error handling
	Retry        *int  `yaml:"retry,omitempty"`
	RetryDelay   *int  `yaml:"retry_delay,omitempty"`
	AllowFailure *bool `yaml:"allow_failure,omitempty"`
}

// TemplateContainerSpec represents a container in a template
type TemplateContainerSpec struct {
	Name    string                      `yaml:"name"`
	Image   string                      `yaml:"image"`
	Command []string                    `yaml:"command,omitempty"`
	Volumes []TemplateVolumeMount       `yaml:"volumes,omitempty"`
}

// TemplateVolumeMount represents a volume mount in a template
type TemplateVolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mount_path"`
	ReadOnly  bool   `yaml:"read_only,omitempty"`
}

// TemplateArtifactSpec represents an artifact in a template
type TemplateArtifactSpec struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// TemplateArtifactMount represents an artifact mount in a template
type TemplateArtifactMount struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// TemplateInclude represents an included template in a workflow
type TemplateInclude struct {
	Template   string            `yaml:"template"`
	Parameters map[string]string `yaml:"parameters,omitempty"`
	Prefix     string            `yaml:"prefix,omitempty"` // Optional prefix for job names
}

// ParameterSubstitution holds parameter values for substitution
type ParameterSubstitution struct {
	Parameters map[string]string
}

// ProcessedTemplate represents a template after parameter substitution
type ProcessedTemplate struct {
	Steps map[string]SimpleStepSpec
}

// SimpleStepSpec represents a processed step (equivalent to workflow.SimpleStepSpec)
type SimpleStepSpec struct {
	Summary       string                `yaml:"summary,omitempty"`
	Image         string                `yaml:"image"`
	Command       []string              `yaml:"command,omitempty"`
	Containers    []SimpleContainerSpec `yaml:"containers,omitempty"`
	DependsOn     []string              `yaml:"depends_on,omitempty"`
	Env           map[string]string     `yaml:"env,omitempty"`
	Variables     map[string]string     `yaml:"variables,omitempty"`
	WorkDir       string                `yaml:"workdir,omitempty"`
	When          interface{}           `yaml:"when,omitempty"`
	Artifacts     []SimpleArtifactSpec  `yaml:"artifacts,omitempty"`
	UsesArtifacts []SimpleArtifactMount `yaml:"uses_artifacts,omitempty"`
	
	// Error handling
	Retry        *int  `yaml:"retry,omitempty"`
	RetryDelay   *int  `yaml:"retry_delay,omitempty"`
	AllowFailure *bool `yaml:"allow_failure,omitempty"`
}

// SimpleContainerSpec represents a container in a processed step
type SimpleContainerSpec struct {
	Name    string              `yaml:"name"`
	Image   string              `yaml:"image"`
	Command []string            `yaml:"command,omitempty"`
	Volumes []SimpleVolumeMount `yaml:"volumes,omitempty"`
}

// SimpleVolumeMount represents a volume mount in a processed step
type SimpleVolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mount_path"`
	ReadOnly  bool   `yaml:"read_only,omitempty"`
}

// SimpleArtifactSpec represents an artifact in a processed step
type SimpleArtifactSpec struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// SimpleArtifactMount represents an artifact mount in a processed step
type SimpleArtifactMount struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}