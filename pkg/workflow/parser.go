package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	
	"basic-container-runtime/pkg/templates"
)

type SimpleWorkflowSpec struct {
	Kind      string                         `yaml:"kind"`
	Version   int                            `yaml:"version"`
	Variables map[string]string              `yaml:"variables,omitempty"`
	Secrets   []string                       `yaml:"secrets,omitempty"`
	Volumes   map[string]SimpleVolumeSpec    `yaml:"volumes,omitempty"`
	Includes  []templates.TemplateInclude    `yaml:"includes,omitempty"`
	Steps     map[string]SimpleStepSpec      `yaml:"steps"`
}

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
	RetryDelay   *int  `yaml:"retry_delay,omitempty"` // seconds
	AllowFailure *bool `yaml:"allow_failure,omitempty"`
}

type SimpleContainerSpec struct {
	Name    string              `yaml:"name"`
	Image   string              `yaml:"image"`
	Command []string            `yaml:"command,omitempty"`
	Volumes []SimpleVolumeMount `yaml:"volumes,omitempty"`
}

type SimpleVolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mount_path"`
	ReadOnly  bool   `yaml:"read_only,omitempty"`
}

type SimpleVolumeSpec struct {
	Type     string `yaml:"type"`
	HostPath string `yaml:"host_path,omitempty"`
	Size     string `yaml:"size,omitempty"`
}

type SimpleArtifactSpec struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type SimpleArtifactMount struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

func ParseSimpleWorkflow(filename string) (*SimpleWorkflowSpec, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	return ParseSimpleWorkflowFromBytes(data)
}

// ParseSimpleWorkflowFromString parses workflow from YAML string
func ParseSimpleWorkflowFromString(yamlContent string) (*SimpleWorkflowSpec, error) {
	return ParseSimpleWorkflowFromBytes([]byte(yamlContent))
}

// ParseSimpleWorkflowFromBytes parses workflow from YAML bytes
func ParseSimpleWorkflowFromBytes(data []byte) (*SimpleWorkflowSpec, error) {
	var spec SimpleWorkflowSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if spec.Kind != "workflow" {
		return nil, fmt.Errorf("invalid kind: expected 'workflow', got '%s'", spec.Kind)
	}

	return &spec, nil
}

func parseCondition(whenValue interface{}) (Condition, error) {
	if whenValue == nil {
		return nil, nil
	}

	if str, ok := whenValue.(string); ok {
		switch strings.ToLower(strings.TrimSpace(str)) {
		case "always":
			return AlwaysCondition{}, nil
		case "never":
			return NeverCondition{}, nil
		default:
			return nil, fmt.Errorf("unknown string condition: %s", str)
		}
	}

	if condMap, ok := whenValue.(map[string]interface{}); ok {
		if variable, hasVar := condMap["variable"]; hasVar {
			if equals, hasEquals := condMap["equals"]; hasEquals {
				varStr, varOk := variable.(string)
				equalsStr, equalsOk := equals.(string)
				if !varOk || !equalsOk {
					return nil, fmt.Errorf("variable and equals must be strings")
				}
				return VariableCondition{
					Variable: varStr,
					Equals:   equalsStr,
				}, nil
			}
			return nil, fmt.Errorf("variable condition must have 'equals' field")
		}

		if job, hasJob := condMap["job"]; hasJob {
			if status, hasStatus := condMap["status"]; hasStatus {
				jobStr, jobOk := job.(string)
				statusStr, statusOk := status.(string)
				if !jobOk || !statusOk {
					return nil, fmt.Errorf("job and status must be strings")
				}
				return JobStatusCondition{
					Job:    jobStr,
					Status: statusStr,
				}, nil
			}
			return nil, fmt.Errorf("job condition must have 'status' field")
		}

		return nil, fmt.Errorf("unknown condition map structure")
	}

	return nil, fmt.Errorf("invalid condition type: %T", whenValue)
}

func ConvertToWorkflow(spec *SimpleWorkflowSpec, filename string) *Workflow {
	workflow := &Workflow{
		ID:      "simple-workflow",
		Name:    "Simple Workflow",
		Version: fmt.Sprintf("v%d", spec.Version),
		Jobs:    make([]Job, 0),
		Status:  WorkflowStatusPending,
	}

	if spec.Variables != nil {
		workflow.Variables = spec.Variables
	}

	if spec.Secrets != nil {
		workflow.Secrets = spec.Secrets
	}

	if spec.Volumes != nil {
		workflow.Volumes = make(map[string]VolumeSpec)
		for volumeName, volumeSpec := range spec.Volumes {
			workflow.Volumes[volumeName] = VolumeSpec{
				Name:     volumeName,
				Type:     volumeSpec.Type,
				HostPath: volumeSpec.HostPath,
				Size:     volumeSpec.Size,
			}
		}
	}

	// Process template includes
	baseDir := filepath.Dir(filename) // Get directory of the main workflow file
	allSteps := make(map[string]SimpleStepSpec)
	
	// First, add steps from included templates
	for _, include := range spec.Includes {
		processedSteps, err := processTemplateInclude(include, baseDir)
		if err != nil {
			// Return a minimal workflow to avoid panic, but log the error
			fmt.Printf("Error processing template include '%s': %v\n", include.Template, err)
			continue
		}
		
		// Add processed steps with optional prefix
		for stepName, stepSpec := range processedSteps {
			finalStepName := stepName
			if include.Prefix != "" {
				finalStepName = include.Prefix + "-" + stepName
				
				// Also adjust dependencies to include the prefix
				if len(stepSpec.DependsOn) > 0 {
					adjustedDeps := make([]string, len(stepSpec.DependsOn))
					for i, dep := range stepSpec.DependsOn {
						// Check if dependency is from the same template (exists in processedSteps)
						if _, exists := processedSteps[dep]; exists {
							adjustedDeps[i] = include.Prefix + "-" + dep
						} else {
							adjustedDeps[i] = dep // External dependency, keep as-is
						}
					}
					stepSpec.DependsOn = adjustedDeps
				}
			}
			allSteps[finalStepName] = stepSpec
		}
	}
	
	// Then, add steps from the main workflow
	for stepName, stepSpec := range spec.Steps {
		allSteps[stepName] = stepSpec
	}

	jobIndex := 0
	for stepName, stepSpec := range allSteps {
		job := Job{
			ID:          fmt.Sprintf("job-%d", jobIndex),
			Name:        stepName,
			Description: stepSpec.Summary,
			DependsOn:   stepSpec.DependsOn,
			Commands:    make([]Command, 1),
			Status:      JobStatusPending,
		}

		job.Commands[0] = Command{
			ID:     fmt.Sprintf("cmd-%d-0", jobIndex),
			Status: CommandStatusPending,
		}

		if len(stepSpec.Containers) > 0 {
			job.Containers = make([]ContainerSpec, len(stepSpec.Containers))
			for i, containerSpec := range stepSpec.Containers {
				container := ContainerSpec{
					Name:    containerSpec.Name,
					Image:   containerSpec.Image,
					Command: containerSpec.Command,
				}

				if len(containerSpec.Volumes) > 0 {
					container.Volumes = make([]VolumeMount, len(containerSpec.Volumes))
					for j, volumeMount := range containerSpec.Volumes {
						container.Volumes[j] = VolumeMount{
							Name:        volumeMount.Name,
							MountPath:   volumeMount.MountPath,
							Destination: volumeMount.MountPath,
							ReadOnly:    volumeMount.ReadOnly,
						}
					}
				}

				job.Containers[i] = container
			}
		} else {
			job.Container = &ContainerSpec{
				Image:   stepSpec.Image,
				Command: stepSpec.Command,
			}
		}

		if stepSpec.Env != nil {
			job.Environment = stepSpec.Env
		}

		if stepSpec.Variables != nil {
			job.Variables = stepSpec.Variables
		}

		if stepSpec.WorkDir != "" {
			job.WorkingDir = stepSpec.WorkDir
		}

		if stepSpec.When != nil {
			condition, err := parseCondition(stepSpec.When)
			if err != nil {
				fmt.Printf("Warning: failed to parse condition for job %s: %v\n", stepName, err)
			} else {
				job.When = condition
			}
		}

		if len(stepSpec.Artifacts) > 0 {
			job.Artifacts = make([]ArtifactSpec, len(stepSpec.Artifacts))
			for i, artifactSpec := range stepSpec.Artifacts {
				job.Artifacts[i] = ArtifactSpec{
					Name: artifactSpec.Name,
					Path: artifactSpec.Path,
				}
			}
		}

		if len(stepSpec.UsesArtifacts) > 0 {
			job.UsesArtifacts = make([]ArtifactMount, len(stepSpec.UsesArtifacts))
			for i, artifactMount := range stepSpec.UsesArtifacts {
				job.UsesArtifacts[i] = ArtifactMount{
					Name: artifactMount.Name,
					Path: artifactMount.Path,
				}
			}
		}

		// Parse error handling settings
		if stepSpec.AllowFailure != nil {
			job.AllowFailure = *stepSpec.AllowFailure
		}
		
		if stepSpec.Retry != nil || stepSpec.RetryDelay != nil {
			job.RetryPolicy = &RetryPolicy{}
			if stepSpec.Retry != nil {
				job.RetryPolicy.MaxAttempts = *stepSpec.Retry
			}
			if stepSpec.RetryDelay != nil {
				delayDuration := time.Duration(*stepSpec.RetryDelay) * time.Second
				job.RetryPolicy.InitialDelay = &delayDuration
			}
		}

		workflow.Jobs = append(workflow.Jobs, job)
		jobIndex++
	}

	return workflow
}

// processTemplateInclude processes a single template include
func processTemplateInclude(include templates.TemplateInclude, baseDir string) (map[string]SimpleStepSpec, error) {
	// Resolve template path
	templatePath := templates.ResolveTemplatePath(include.Template, baseDir)
	
	// Parse template
	template, err := templates.ParseTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	
	// Process template with parameters
	processedTemplate, err := templates.ProcessTemplate(template, include.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to process template: %w", err)
	}
	
	// Convert from templates.SimpleStepSpec to workflow.SimpleStepSpec
	workflowSteps := make(map[string]SimpleStepSpec)
	for stepName, templateStep := range processedTemplate.Steps {
		workflowStep := convertTemplateStepToWorkflowStep(templateStep)
		workflowSteps[stepName] = workflowStep
	}
	
	return workflowSteps, nil
}

// convertTemplateStepToWorkflowStep converts a templates.SimpleStepSpec to workflow.SimpleStepSpec
func convertTemplateStepToWorkflowStep(templateStep templates.SimpleStepSpec) SimpleStepSpec {
	step := SimpleStepSpec{
		Summary:      templateStep.Summary,
		Image:        templateStep.Image,
		Command:      templateStep.Command,
		DependsOn:    templateStep.DependsOn,
		Env:          templateStep.Env,
		Variables:    templateStep.Variables,
		WorkDir:      templateStep.WorkDir,
		When:         templateStep.When,
		Retry:        templateStep.Retry,
		RetryDelay:   templateStep.RetryDelay,
		AllowFailure: templateStep.AllowFailure,
	}

	// Convert containers
	if len(templateStep.Containers) > 0 {
		step.Containers = make([]SimpleContainerSpec, len(templateStep.Containers))
		for i, container := range templateStep.Containers {
			step.Containers[i] = SimpleContainerSpec{
				Name:    container.Name,
				Image:   container.Image,
				Command: container.Command,
			}
			
			// Convert volumes
			if len(container.Volumes) > 0 {
				step.Containers[i].Volumes = make([]SimpleVolumeMount, len(container.Volumes))
				for j, volume := range container.Volumes {
					step.Containers[i].Volumes[j] = SimpleVolumeMount{
						Name:      volume.Name,
						MountPath: volume.MountPath,
						ReadOnly:  volume.ReadOnly,
					}
				}
			}
		}
	}

	// Convert artifacts
	if len(templateStep.Artifacts) > 0 {
		step.Artifacts = make([]SimpleArtifactSpec, len(templateStep.Artifacts))
		for i, artifact := range templateStep.Artifacts {
			step.Artifacts[i] = SimpleArtifactSpec{
				Name: artifact.Name,
				Path: artifact.Path,
			}
		}
	}

	// Convert uses_artifacts
	if len(templateStep.UsesArtifacts) > 0 {
		step.UsesArtifacts = make([]SimpleArtifactMount, len(templateStep.UsesArtifacts))
		for i, artifact := range templateStep.UsesArtifacts {
			step.UsesArtifacts[i] = SimpleArtifactMount{
				Name: artifact.Name,
				Path: artifact.Path,
			}
		}
	}

	return step
}
