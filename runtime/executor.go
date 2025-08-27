package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"

	"basic-container-runtime/pkg/workflow"
)

type ContainerExecutor struct {
	client          *client.Client
	logger          *logrus.Logger
	volumeManager   *VolumeManager
	imageCache      *ImageCache
	artifactManager *ArtifactManager
}

func NewContainerExecutor() (*ContainerExecutor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	logger := logrus.New()
	volumeManager, err := NewVolumeManager(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume manager: %w", err)
	}

	imageCache := NewImageCache(logger)
	
	artifactManager, err := NewArtifactManager(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact manager: %w", err)
	}

	return &ContainerExecutor{
		client:          cli,
		logger:          logger,
		volumeManager:   volumeManager,
		imageCache:      imageCache,
		artifactManager: artifactManager,
	}, nil
}

func (e *ContainerExecutor) ExecuteJob(ctx context.Context, job *workflow.Job, workflowVariables map[string]string) error {
	e.logger.Infof("Starting job: %s", job.Name)
	e.logger.Debugf("Job %s has %d containers, %d dependencies", job.Name, len(job.Containers), len(job.DependsOn))

	job.Status = workflow.JobStatusRunning
	now := time.Now()
	job.StartedAt = &now

	// Execute job with retry logic
	err := e.executeJobWithRetry(ctx, job, workflowVariables)
	
	if err != nil {
		job.Status = workflow.JobStatusFailed
		finishedAt := time.Now()
		job.FinishedAt = &finishedAt
		
		if job.AllowFailure {
			e.logger.Warnf("Job %s failed but marked as allowed to fail: %v", job.Name, err)
			job.Status = workflow.JobStatusCompleted
			return nil
		}
		
		return err
	}

	job.Status = workflow.JobStatusCompleted
	finishedAt := time.Now()
	job.FinishedAt = &finishedAt

	e.logger.Infof("Job completed: %s", job.Name)
	return nil
}

func (e *ContainerExecutor) executeJobWithRetry(ctx context.Context, job *workflow.Job, workflowVariables map[string]string) error {
	maxAttempts := 1
	var delay time.Duration
	
	if job.RetryPolicy != nil {
		if job.RetryPolicy.MaxAttempts > 0 {
			maxAttempts = job.RetryPolicy.MaxAttempts
		}
		if job.RetryPolicy.InitialDelay != nil {
			delay = *job.RetryPolicy.InitialDelay
		}
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			e.logger.Infof("Job %s - Retry attempt %d/%d", job.Name, attempt, maxAttempts)
			if delay > 0 {
				e.logger.Debugf("Waiting %v before retry", delay)
				time.Sleep(delay)
			}
		} else {
			e.logger.Infof("Job %s - Attempt %d/%d", job.Name, attempt, maxAttempts)
		}

		err := e.executeJobCore(ctx, job, workflowVariables)
		if err == nil {
			if attempt > 1 {
				e.logger.Infof("Job %s succeeded on attempt %d/%d", job.Name, attempt, maxAttempts)
			}
			return nil
		}

		e.logger.Warnf("Job %s failed on attempt %d/%d: %v", job.Name, attempt, maxAttempts, err)
		
		if attempt == maxAttempts {
			return fmt.Errorf("job failed after %d attempts: %w", maxAttempts, err)
		}
	}
	
	return nil
}

func (e *ContainerExecutor) executeJobCore(ctx context.Context, job *workflow.Job, workflowVariables map[string]string) error {
	if len(job.Containers) > 0 {
		if err := e.executeMultipleContainers(ctx, job, workflowVariables); err != nil {
			return fmt.Errorf("multi-container execution failed: %w", err)
		}
	} else {
		if job.Container == nil {
			return fmt.Errorf("no container specification found for job %s", job.Name)
		}

		for i := range job.Commands {
			if err := e.executeCommand(ctx, job, &job.Commands[i], workflowVariables); err != nil {
				return fmt.Errorf("command failed: %w", err)
			}
		}
	}
	
	return nil
}

func (e *ContainerExecutor) PrintCacheStats() {
	if e.imageCache != nil {
		e.imageCache.PrintStats()
		cachedImages := e.imageCache.ListCachedImages()
		if len(cachedImages) > 0 {
			e.logger.Debugf("Cached images: %v", cachedImages)
		}
	}
}

func (e *ContainerExecutor) Close() error {
	if e.volumeManager != nil {
		if err := e.volumeManager.Cleanup(); err != nil {
			e.logger.Warnf("Failed to cleanup volumes: %v", err)
		}
	}

	if e.artifactManager != nil {
		if err := e.artifactManager.Cleanup(); err != nil {
			e.logger.Warnf("Failed to cleanup artifacts: %v", err)
		}
	}

	if e.client != nil {
		return e.client.Close()
	}
	return nil
}