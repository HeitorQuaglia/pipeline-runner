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
	client        *client.Client
	logger        *logrus.Logger
	volumeManager *VolumeManager
	imageCache    *ImageCache
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

	return &ContainerExecutor{
		client:        cli,
		logger:        logger,
		volumeManager: volumeManager,
		imageCache:    imageCache,
	}, nil
}

func (e *ContainerExecutor) ExecuteJob(ctx context.Context, job *workflow.Job, workflowVariables map[string]string) error {
	e.logger.Infof("Starting job: %s", job.Name)
	e.logger.Debugf("Job %s has %d containers, %d dependencies", job.Name, len(job.Containers), len(job.DependsOn))

	job.Status = workflow.JobStatusRunning
	now := time.Now()
	job.StartedAt = &now

	if len(job.Containers) > 0 {
		if err := e.executeMultipleContainers(ctx, job, workflowVariables); err != nil {
			job.Status = workflow.JobStatusFailed
			finishedAt := time.Now()
			job.FinishedAt = &finishedAt
			return fmt.Errorf("multi-container execution failed: %w", err)
		}
	} else {
		if job.Container == nil {
			return fmt.Errorf("no container specification found for job %s", job.Name)
		}

		for i := range job.Commands {
			if err := e.executeCommand(ctx, job, &job.Commands[i], workflowVariables); err != nil {
				job.Status = workflow.JobStatusFailed
				finishedAt := time.Now()
				job.FinishedAt = &finishedAt
				return fmt.Errorf("command failed: %w", err)
			}
		}
	}

	job.Status = workflow.JobStatusCompleted
	finishedAt := time.Now()
	job.FinishedAt = &finishedAt

	e.logger.Infof("Job completed: %s", job.Name)
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

	if e.client != nil {
		return e.client.Close()
	}
	return nil
}