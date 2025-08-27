package workflow

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type SimpleWorkflowSpec struct {
	Kind      string                      `yaml:"kind"`
	Version   int                         `yaml:"version"`
	Variables map[string]string           `yaml:"variables,omitempty"`
	Secrets   []string                    `yaml:"secrets,omitempty"`
	Volumes   map[string]SimpleVolumeSpec `yaml:"volumes,omitempty"`
	Steps     map[string]SimpleStepSpec   `yaml:"steps"`
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

func ConvertToWorkflow(spec *SimpleWorkflowSpec) *Workflow {
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

	jobIndex := 0
	for stepName, stepSpec := range spec.Steps {
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
