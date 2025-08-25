package workflow

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type SimpleWorkflowSpec struct {
	Kind    string                    `yaml:"kind"`
	Version int                       `yaml:"version"`
	Steps   map[string]SimpleStepSpec `yaml:"steps"`
}

type SimpleStepSpec struct {
	Summary string `yaml:"summary,omitempty"`
	Image   string `yaml:"image"`
	Command []string `yaml:"command,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	WorkDir string `yaml:"workdir,omitempty"`
}

func ParseSimpleWorkflow(filename string) (*SimpleWorkflowSpec, error) {
	data, err := ioutil.ReadFile(filename)
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
			Commands:    make([]Command, 1),
			Container: &ContainerSpec{
				Image: stepSpec.Image,
			},
			Status: JobStatusPending,
		}

		if len(stepSpec.Command) > 0 {
			job.Commands[0] = Command{
				ID:     fmt.Sprintf("cmd-%d-0", jobIndex),
				Script: fmt.Sprintf("%v", stepSpec.Command),
				Status: CommandStatusPending,
			}
		} else {
			job.Commands[0] = Command{
				ID:     fmt.Sprintf("cmd-%d-0", jobIndex),
				Script: "",
				Status: CommandStatusPending,
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