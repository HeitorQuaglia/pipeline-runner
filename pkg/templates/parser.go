package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseTemplate loads and parses a template file
func ParseTemplate(filename string) (*Template, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", filename, err)
	}

	var template Template
	if err := yaml.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("failed to parse template YAML: %w", err)
	}

	if template.Kind != "template" {
		return nil, fmt.Errorf("invalid kind: expected 'template', got '%s'", template.Kind)
	}

	return &template, nil
}

// ProcessTemplate applies parameter substitution to a template
func ProcessTemplate(template *Template, parameters map[string]string) (*ProcessedTemplate, error) {
	// Build final parameter map with defaults
	finalParams := make(map[string]string)
	
	// Start with defaults
	for _, param := range template.Parameters {
		if param.Default != "" {
			finalParams[param.Name] = param.Default
		}
	}
	
	// Override with provided parameters
	for key, value := range parameters {
		finalParams[key] = value
	}
	
	// Validate required parameters
	for _, param := range template.Parameters {
		if param.Required && finalParams[param.Name] == "" {
			return nil, fmt.Errorf("required parameter '%s' not provided", param.Name)
		}
	}

	// Process steps with parameter substitution
	processedSteps := make(map[string]SimpleStepSpec)
	
	for stepName, templateStep := range template.Steps {
		processedStep, err := processTemplateStep(&templateStep, finalParams)
		if err != nil {
			return nil, fmt.Errorf("failed to process step '%s': %w", stepName, err)
		}
		processedSteps[stepName] = *processedStep
	}

	return &ProcessedTemplate{
		Steps: processedSteps,
	}, nil
}

// processTemplateStep processes a single template step with parameter substitution
func processTemplateStep(templateStep *TemplateStep, parameters map[string]string) (*SimpleStepSpec, error) {
	step := &SimpleStepSpec{
		Summary:   substituteParameters(templateStep.Summary, parameters),
		Image:     substituteParameters(templateStep.Image, parameters),
		DependsOn: substituteParametersInSlice(templateStep.DependsOn, parameters),
		WorkDir:   substituteParameters(templateStep.WorkDir, parameters),
	}

	// Process command
	if len(templateStep.Command) > 0 {
		step.Command = substituteParametersInSlice(templateStep.Command, parameters)
	}

	// Process environment variables
	if templateStep.Env != nil {
		step.Env = make(map[string]string)
		for key, value := range templateStep.Env {
			step.Env[substituteParameters(key, parameters)] = substituteParameters(value, parameters)
		}
	}

	// Process variables
	if templateStep.Variables != nil {
		step.Variables = make(map[string]string)
		for key, value := range templateStep.Variables {
			step.Variables[substituteParameters(key, parameters)] = substituteParameters(value, parameters)
		}
	}

	// Process containers
	if len(templateStep.Containers) > 0 {
		step.Containers = make([]SimpleContainerSpec, len(templateStep.Containers))
		for i, container := range templateStep.Containers {
			processedContainer := SimpleContainerSpec{
				Name:    substituteParameters(container.Name, parameters),
				Image:   substituteParameters(container.Image, parameters),
				Command: substituteParametersInSlice(container.Command, parameters),
			}
			
			// Process volumes
			if len(container.Volumes) > 0 {
				processedContainer.Volumes = make([]SimpleVolumeMount, len(container.Volumes))
				for j, volume := range container.Volumes {
					processedContainer.Volumes[j] = SimpleVolumeMount{
						Name:      substituteParameters(volume.Name, parameters),
						MountPath: substituteParameters(volume.MountPath, parameters),
						ReadOnly:  volume.ReadOnly,
					}
				}
			}
			
			step.Containers[i] = processedContainer
		}
	}

	// Process artifacts
	if len(templateStep.Artifacts) > 0 {
		step.Artifacts = make([]SimpleArtifactSpec, len(templateStep.Artifacts))
		for i, artifact := range templateStep.Artifacts {
			step.Artifacts[i] = SimpleArtifactSpec{
				Name: substituteParameters(artifact.Name, parameters),
				Path: substituteParameters(artifact.Path, parameters),
			}
		}
	}

	// Process uses_artifacts
	if len(templateStep.UsesArtifacts) > 0 {
		step.UsesArtifacts = make([]SimpleArtifactMount, len(templateStep.UsesArtifacts))
		for i, artifact := range templateStep.UsesArtifacts {
			step.UsesArtifacts[i] = SimpleArtifactMount{
				Name: substituteParameters(artifact.Name, parameters),
				Path: substituteParameters(artifact.Path, parameters),
			}
		}
	}

	// Process error handling
	step.Retry = templateStep.Retry
	step.RetryDelay = templateStep.RetryDelay
	step.AllowFailure = templateStep.AllowFailure
	step.When = templateStep.When

	return step, nil
}

// substituteParameters replaces {{PARAM_NAME}} with actual values
func substituteParameters(input string, parameters map[string]string) string {
	if input == "" {
		return input
	}

	// Regular expression to match {{PARAM_NAME}}
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	
	return re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract parameter name (remove {{ and }})
		paramName := strings.TrimSpace(match[2 : len(match)-2])
		
		if value, exists := parameters[paramName]; exists {
			return value
		}
		
		// If parameter not found, return original (could be an error in production)
		return match
	})
}

// substituteParametersInSlice applies parameter substitution to a slice of strings
func substituteParametersInSlice(input []string, parameters map[string]string) []string {
	if len(input) == 0 {
		return input
	}

	result := make([]string, len(input))
	for i, str := range input {
		result[i] = substituteParameters(str, parameters)
	}
	return result
}

// ResolveTemplatePath resolves a template path relative to a base directory
func ResolveTemplatePath(templatePath, baseDir string) string {
	if filepath.IsAbs(templatePath) {
		return templatePath
	}
	return filepath.Join(baseDir, templatePath)
}