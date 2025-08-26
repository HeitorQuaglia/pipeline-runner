package workflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type SimpleWorkflowSpec struct {
	Kind      string                      `yaml:"kind"`
	Version   int                         `yaml:"version"`
	Variables map[string]string           `yaml:"variables,omitempty"`
	Volumes   map[string]SimpleVolumeSpec `yaml:"volumes,omitempty"`
	Steps     map[string]SimpleStepSpec   `yaml:"steps"`
}

type SimpleStepSpec struct {
	Summary    string                `yaml:"summary,omitempty"`
	Image      string                `yaml:"image"`
	Command    []string              `yaml:"command,omitempty"`
	Containers []SimpleContainerSpec `yaml:"containers,omitempty"`
	Env        map[string]string     `yaml:"env,omitempty"`
	Variables  map[string]string     `yaml:"variables,omitempty"`
	WorkDir    string                `yaml:"workdir,omitempty"`
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

	if spec.Variables != nil {
		workflow.Variables = spec.Variables
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

		workflow.Jobs = append(workflow.Jobs, job)
		jobIndex++
	}

	return workflow
}
