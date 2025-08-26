package runtime

import (
	"fmt"

	"basic-container-runtime/pkg/workflow"
)

func (e *ContainerExecutor) buildEnvVars(job *workflow.Job, workflowVariables map[string]string) []string {
	var envVars []string

	for key, value := range workflowVariables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	for key, value := range job.Variables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	for key, value := range job.Environment {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	if job.Container != nil && job.Container.Environment != nil {
		for key, value := range job.Container.Environment {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return envVars
}

func (e *ContainerExecutor) buildEnvVarsForContainer(job *workflow.Job, containerSpec *workflow.ContainerSpec, workflowVariables map[string]string) []string {
	var envVars []string

	for key, value := range workflowVariables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	for key, value := range job.Variables {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	for key, value := range job.Environment {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	if containerSpec.Environment != nil {
		for key, value := range containerSpec.Environment {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return envVars
}