package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type Engine struct {
	logger *logrus.Logger
}

func NewEngine() *Engine {
	return &Engine{
		logger: logrus.New(),
	}
}

func (e *Engine) ExecuteWorkflow(ctx context.Context, workflow *Workflow, executor JobExecutor) error {
	e.logger.Infof("Starting workflow execution: %s", workflow.Name)
	e.logger.Debugf("Workflow has %d jobs and %d volumes", len(workflow.Jobs), len(workflow.Volumes))

	workflow.Status = WorkflowStatusRunning
	now := time.Now()
	workflow.StartedAt = &now

	if err := executor.InitializeVolumes(workflow.Volumes); err != nil {
		e.logger.Errorf("Failed to initialize volumes: %v", err)
		workflow.Status = WorkflowStatusFailed
		finishedAt := time.Now()
		workflow.FinishedAt = &finishedAt
		return fmt.Errorf("failed to initialize volumes: %w", err)
	}

	scheduler := NewDependencyScheduler(workflow.Jobs, e.logger)
	e.logger.Debugf("Created dependency scheduler for %d jobs", len(workflow.Jobs))

	if err := scheduler.ValidateDependencies(); err != nil {
		e.logger.Errorf("Dependency validation failed: %v", err)
		workflow.Status = WorkflowStatusFailed
		finishedAt := time.Now()
		workflow.FinishedAt = &finishedAt
		return fmt.Errorf("dependency validation failed: %w", err)
	}
	e.logger.Debugf("Dependency validation passed")

	if err := e.executeJobsWithDependencies(ctx, scheduler, executor, workflow.Variables); err != nil {
		e.logger.Errorf("Workflow execution failed: %v", err)
		workflow.Status = WorkflowStatusFailed
		finishedAt := time.Now()
		workflow.FinishedAt = &finishedAt
		return err
	}

	workflow.Status = WorkflowStatusCompleted
	finishedAt := time.Now()
	workflow.FinishedAt = &finishedAt

	e.logger.Infof("Workflow completed successfully: %s", workflow.Name)
	return nil
}


func (e *Engine) ValidateWorkflow(workflow *Workflow) error {
	if workflow.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	if len(workflow.Jobs) == 0 {
		return fmt.Errorf("workflow must have at least one job")
	}

	for _, job := range workflow.Jobs {
		if err := e.validateJob(&job); err != nil {
			return fmt.Errorf("job %s validation failed: %w", job.Name, err)
		}
	}

	scheduler := NewDependencyScheduler(workflow.Jobs, e.logger)
	if err := scheduler.ValidateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	return nil
}

func (e *Engine) validateJob(job *Job) error {
	if job.Name == "" {
		return fmt.Errorf("job name is required")
	}

	if len(job.Commands) == 0 {
		return fmt.Errorf("job must have at least one command")
	}

	if len(job.Containers) > 0 {
		for i, container := range job.Containers {
			if container.Image == "" {
				return fmt.Errorf("container %d image is required", i)
			}
			if container.Name == "" {
				return fmt.Errorf("container %d name is required", i)
			}
		}
	} else {
		if job.Container == nil {
			return fmt.Errorf("job must have a container specification")
		}

		if job.Container.Image == "" {
			return fmt.Errorf("container image is required")
		}
	}

	return nil
}
