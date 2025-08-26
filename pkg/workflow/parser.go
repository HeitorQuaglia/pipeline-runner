package workflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type SimpleWorkflowSpec struct {
	Kind    string                    `yaml:"kind"`
	Version int                       `yaml:"version"`
	Steps   map[string]SimpleStepSpec `yaml:"steps"`
}

type SimpleStepSpec struct {
	Summary    string                  `yaml:"summary,omitempty"`
	Image      string                  `yaml:"image"`
	Command    []string                `yaml:"command,omitempty"`
	Containers []SimpleContainerSpec   `yaml:"containers,omitempty"`
	Env        map[string]string       `yaml:"env,omitempty"`
	WorkDir    string                  `yaml:"workdir,omitempty"`
}

type SimpleContainerSpec struct {
	Name    string   `yaml:"name"`
	Image   string   `yaml:"image"`
	Command []string `yaml:"command,omitempty"`
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

func ConvertToWorkflow(spec *SimpleWorkflowSpec) *Workflow {
	workflow := &Workflow{
		ID:      "simple-workflow",
		Name:    "Simple Workflow",
		Version: fmt.Sprintf("v%d", spec.Version),
		Jobs:    make([]Job, 0),
		Status:  WorkflowStatusPending,
	}

	jobIndex := 0
	for stepName, stepSpec := range spec.Steps {
		job := Job{
			ID:          fmt.Sprintf("job-%d", jobIndex),
			Name:        stepName,
			Description: stepSpec.Summary,
			Commands:    make([]Command, 1), // Dummy command for validation
			Status:      JobStatusPending,
		}

		// Add a dummy command to pass validation in engine
		job.Commands[0] = Command{
			ID:     fmt.Sprintf("cmd-%d-0", jobIndex),
			Status: CommandStatusPending,
		}

		// Check if containers are defined (multi-container format)
		if len(stepSpec.Containers) > 0 {
			// Multi-container format
			job.Containers = make([]ContainerSpec, len(stepSpec.Containers))
			for i, containerSpec := range stepSpec.Containers {
				job.Containers[i] = ContainerSpec{
					Name:    containerSpec.Name,
					Image:   containerSpec.Image,
					Command: containerSpec.Command,
				}
			}
		} else {
			// Single container format (backward compatibility)
			job.Container = &ContainerSpec{
				Image:   stepSpec.Image,
				Command: stepSpec.Command,
			}
		}

		if stepSpec.Env != nil {
			job.Environment = stepSpec.Env
		}

		if stepSpec.WorkDir != "" {
			job.WorkingDir = stepSpec.WorkDir
		}

		workflow.Jobs = append(workflow.Jobs, job)
		jobIndex++
	}

	return workflow
}