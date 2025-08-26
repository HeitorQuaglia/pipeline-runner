package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sirupsen/logrus"

	"basic-container-runtime/pkg/workflow"
)

type ContainerExecutor struct {
	client *client.Client
	logger *logrus.Logger
}

func NewContainerExecutor() (*ContainerExecutor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &ContainerExecutor{
		client: cli,
		logger: logrus.New(),
	}, nil
}

func (e *ContainerExecutor) ExecuteJob(ctx context.Context, job *workflow.Job) error {
	e.logger.Infof("Starting job: %s", job.Name)

	job.Status = workflow.JobStatusRunning
	now := time.Now()
	job.StartedAt = &now

	if len(job.Containers) > 0 {
		if err := e.executeMultipleContainers(ctx, job); err != nil {
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
			if err := e.executeCommand(ctx, job, &job.Commands[i]); err != nil {
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

func (e *ContainerExecutor) executeMultipleContainers(ctx context.Context, job *workflow.Job) error {
	e.logger.Infof("Executing %d containers for job: %s", len(job.Containers), job.Name)

	var wg sync.WaitGroup
	errCh := make(chan error, len(job.Containers))

	for i := range job.Containers {
		wg.Add(1)
		go func(containerSpec *workflow.ContainerSpec) {
			defer wg.Done()
			if err := e.executeContainer(ctx, job, containerSpec); err != nil {
				errCh <- fmt.Errorf("container %s failed: %w", containerSpec.Name, err)
			}
		}(&job.Containers[i])
	}

	wg.Wait()
	close(errCh)

	var errors []string
	for err := range errCh {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("container execution errors: %s", strings.Join(errors, "; "))
	}

	e.logger.Infof("All containers completed successfully for job: %s", job.Name)
	return nil
}

func (e *ContainerExecutor) executeContainer(ctx context.Context, job *workflow.Job, containerSpec *workflow.ContainerSpec) error {
	e.logger.Infof("Executing container %s in job %s", containerSpec.Name, job.Name)

	image := containerSpec.Image
	if containerSpec.Tag != "" {
		image = fmt.Sprintf("%s:%s", image, containerSpec.Tag)
	}

	if err := e.pullImage(ctx, image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	containerConfig := &container.Config{
		Image: image,
		Env:   e.buildEnvVarsForContainer(job, containerSpec),
		Cmd:   containerSpec.Command,
	}

	if containerSpec.WorkingDir != "" {
		containerConfig.WorkingDir = containerSpec.WorkingDir
	} else if job.WorkingDir != "" {
		containerConfig.WorkingDir = job.WorkingDir
	}

	resp, err := e.client.ContainerCreate(ctx, containerConfig, nil, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	defer func() {
		if err := e.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
			e.logger.Warnf("Failed to remove container: %v", err)
		}
	}()

	if err := e.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	output, err := e.getContainerLogs(ctx, resp.ID)
	if err != nil {
		e.logger.Warnf("Failed to get container logs: %v", err)
	}

	if output != "" {
		e.logger.Infof("Container output for %s:", containerSpec.Name)
		fmt.Printf("--- Container Output [%s] ---\n", containerSpec.Name)
		fmt.Print(output)
		fmt.Printf("--- End Output [%s] ---\n", containerSpec.Name)
	}

	statusCh, errCh := e.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container %s failed with exit code %d", containerSpec.Name, status.StatusCode)
		}
	}

	e.logger.Infof("Container %s completed successfully", containerSpec.Name)
	return nil
}

func (e *ContainerExecutor) executeCommand(ctx context.Context, job *workflow.Job, cmd *workflow.Command) error {
	e.logger.Infof("Executing command in job %s", job.Name)

	cmd.Status = workflow.CommandStatusRunning
	now := time.Now()
	cmd.StartedAt = &now

	image := job.Container.Image
	if job.Container.Tag != "" {
		image = fmt.Sprintf("%s:%s", image, job.Container.Tag)
	}

	if err := e.pullImage(ctx, image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	containerConfig := &container.Config{
		Image: image,
		Env:   e.buildEnvVars(job),
		Cmd:   job.Container.Command,
	}

	if job.Container.WorkingDir != "" {
		containerConfig.WorkingDir = job.Container.WorkingDir
	} else if job.WorkingDir != "" {
		containerConfig.WorkingDir = job.WorkingDir
	}

	resp, err := e.client.ContainerCreate(ctx, containerConfig, nil, nil, nil, "")
	if err != nil {
		cmd.Status = workflow.CommandStatusFailed
		return fmt.Errorf("failed to create container: %w", err)
	}

	defer func() {
		if err := e.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
			e.logger.Warnf("Failed to remove container: %v", err)
		}
	}()

	if err := e.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		cmd.Status = workflow.CommandStatusFailed
		return fmt.Errorf("failed to start container: %w", err)
	}

	output, err := e.getContainerLogs(ctx, resp.ID)
	if err != nil {
		e.logger.Warnf("Failed to get container logs: %v", err)
	}
	cmd.Output = output

	if output != "" {
		e.logger.Infof("Container output for job %s:", job.Name)
		fmt.Println("--- Container Output ---")
		fmt.Print(output)
		fmt.Println("--- End Output ---")
	}

	statusCh, errCh := e.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			cmd.Status = workflow.CommandStatusFailed
			return fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		exitCode := int(status.StatusCode)
		cmd.ExitCode = &exitCode

		if exitCode != 0 {
			cmd.Status = workflow.CommandStatusFailed
			return fmt.Errorf("command failed with exit code %d", exitCode)
		}
	}

	cmd.Status = workflow.CommandStatusCompleted
	finishedAt := time.Now()
	cmd.FinishedAt = &finishedAt

	return nil
}

func (e *ContainerExecutor) pullImage(ctx context.Context, image string) error {
	e.logger.Infof("Pulling image: %s", image)

	reader, err := e.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		e.logger.Debug(scanner.Text())
	}

	return scanner.Err()
}

func (e *ContainerExecutor) getContainerLogs(ctx context.Context, containerID string) (string, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	reader, err := e.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var output strings.Builder
	_, err = stdcopy.StdCopy(&output, &output, reader)
	if err != nil && err != io.EOF {
		return "", err
	}

	return output.String(), nil
}

func (e *ContainerExecutor) buildEnvVars(job *workflow.Job) []string {
	var envVars []string

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

func (e *ContainerExecutor) buildEnvVarsForContainer(job *workflow.Job, containerSpec *workflow.ContainerSpec) []string {
	var envVars []string

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

func (e *ContainerExecutor) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}
