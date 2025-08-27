package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/stdcopy"

	"basic-container-runtime/pkg/workflow"
)

func (e *ContainerExecutor) executeMultipleContainers(ctx context.Context, job *workflow.Job, workflowVariables map[string]string) error {
	e.logger.Infof("Executing %d containers for job: %s", len(job.Containers), job.Name)
	e.logger.Debugf("Container names: %v", func() []string {
		var names []string
		for _, c := range job.Containers {
			names = append(names, c.Name)
		}
		return names
	}())

	var wg sync.WaitGroup
	errCh := make(chan error, len(job.Containers))

	for i := range job.Containers {
		wg.Add(1)
		go func(containerSpec *workflow.ContainerSpec) {
			defer wg.Done()
			if err := e.executeContainer(ctx, job, containerSpec, workflowVariables); err != nil {
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

func (e *ContainerExecutor) executeContainer(ctx context.Context, job *workflow.Job, containerSpec *workflow.ContainerSpec, workflowVariables map[string]string) error {
	e.logger.Infof("Executing container %s in job %s", containerSpec.Name, job.Name)
	e.logger.Debugf("Container %s using image %s", containerSpec.Name, containerSpec.Image)

	image := containerSpec.Image
	if containerSpec.Tag != "" {
		image = fmt.Sprintf("%s:%s", image, containerSpec.Tag)
		e.logger.Debugf("Using tagged image: %s", image)
	}

	if err := e.pullImage(ctx, image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	containerConfig := &container.Config{
		Image: image,
		Env:   e.buildEnvVarsForContainer(job, containerSpec, workflowVariables),
		Cmd:   containerSpec.Command,
	}

	hostConfig := &container.HostConfig{}
	var mounts []mount.Mount

	if len(containerSpec.Volumes) > 0 {
		for _, volumeMount := range containerSpec.Volumes {
			hostPath, err := e.volumeManager.EnsureVolume(volumeMount.Name)
			if err != nil {
				return fmt.Errorf("failed to ensure volume %s: %w", volumeMount.Name, err)
			}

			mountType := mount.TypeBind
			mounts = append(mounts, mount.Mount{
				Type:     mountType,
				Source:   hostPath,
				Target:   volumeMount.MountPath,
				ReadOnly: volumeMount.ReadOnly,
			})

			e.logger.Debugf("Mounting volume %s: %s -> %s", volumeMount.Name, hostPath, volumeMount.MountPath)
		}
	}

	if len(job.UsesArtifacts) > 0 {
		e.logger.Debugf("Job %s uses %d artifacts", job.Name, len(job.UsesArtifacts))
		for _, artifactMount := range job.UsesArtifacts {
			artifactPath, exists := e.artifactManager.GetArtifactPath(artifactMount.Name)
			if !exists {
				return fmt.Errorf("artifact %s not found for job %s", artifactMount.Name, job.Name)
			}

			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   artifactPath,
				Target:   artifactMount.Path,
				ReadOnly: true,
			})

			e.logger.Debugf("Mounting artifact %s: %s -> %s", artifactMount.Name, artifactPath, artifactMount.Path)
		}
	}

	if len(mounts) > 0 {
		hostConfig.Mounts = mounts
	}

	if containerSpec.WorkingDir != "" {
		containerConfig.WorkingDir = containerSpec.WorkingDir
	} else if job.WorkingDir != "" {
		containerConfig.WorkingDir = job.WorkingDir
	}

	resp, err := e.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	defer func() {
		if err := e.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); err != nil {
			e.logger.Warnf("Failed to remove container: %v", err)
		}
	}()

	if err := e.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	output, err := e.getContainerLogs(ctx, resp.ID)
	if err != nil {
		e.logger.Warnf("Failed to get container logs: %v", err)
	}

	if output != "" {
		e.logger.Infof("Container output for %s:", containerSpec.Name)
		fmt.Printf("--- Container Output [%s] ---\n", containerSpec.Name)
		
		// Mask secrets in output
		maskedOutput := output
		if e.secretManager != nil {
			maskedOutput = e.secretManager.MaskSecretsInString(output)
		}
		
		fmt.Print(maskedOutput)
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

	if len(job.Artifacts) > 0 {
		e.logger.Debugf("Capturing %d artifacts from job %s", len(job.Artifacts), job.Name)
		for _, artifact := range job.Artifacts {
			if err := e.artifactManager.CopyFromContainer(ctx, e.client, resp.ID, artifact); err != nil {
				e.logger.Warnf("Failed to capture artifact %s: %v", artifact.Name, err)
			}
		}
	}

	e.logger.Infof("Container %s completed successfully", containerSpec.Name)
	return nil
}

func (e *ContainerExecutor) executeCommand(ctx context.Context, job *workflow.Job, cmd *workflow.Command, workflowVariables map[string]string) error {
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
		Env:   e.buildEnvVars(job, workflowVariables),
		Cmd:   job.Container.Command,
	}

	if job.Container.WorkingDir != "" {
		containerConfig.WorkingDir = job.Container.WorkingDir
	} else if job.WorkingDir != "" {
		containerConfig.WorkingDir = job.WorkingDir
	}

	hostConfig := &container.HostConfig{}
	if len(job.UsesArtifacts) > 0 {
		var mounts []mount.Mount
		e.logger.Debugf("Job %s uses %d artifacts", job.Name, len(job.UsesArtifacts))
		for _, artifactMount := range job.UsesArtifacts {
			artifactPath, exists := e.artifactManager.GetArtifactPath(artifactMount.Name)
			if !exists {
				return fmt.Errorf("artifact %s not found for job %s", artifactMount.Name, job.Name)
			}

			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   artifactPath,
				Target:   artifactMount.Path,
				ReadOnly: true,
			})

			e.logger.Debugf("Mounting artifact %s: %s -> %s", artifactMount.Name, artifactPath, artifactMount.Path)
		}
		hostConfig.Mounts = mounts
	}

	resp, err := e.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		cmd.Status = workflow.CommandStatusFailed
		return fmt.Errorf("failed to create container: %w", err)
	}

	defer func() {
		if err := e.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); err != nil {
			e.logger.Warnf("Failed to remove container: %v", err)
		}
	}()

	if err := e.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
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
		
		// Mask secrets in output
		maskedOutput := output
		if e.secretManager != nil {
			maskedOutput = e.secretManager.MaskSecretsInString(output)
		}
		
		fmt.Print(maskedOutput)
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

	if len(job.Artifacts) > 0 {
		e.logger.Debugf("Capturing %d artifacts from job %s", len(job.Artifacts), job.Name)
		for _, artifact := range job.Artifacts {
			if err := e.artifactManager.CopyFromContainer(ctx, e.client, resp.ID, artifact); err != nil {
				e.logger.Warnf("Failed to capture artifact %s: %v", artifact.Name, err)
			}
		}
	}

	cmd.Status = workflow.CommandStatusCompleted
	finishedAt := time.Now()
	cmd.FinishedAt = &finishedAt

	return nil
}

func (e *ContainerExecutor) pullImage(ctx context.Context, imageName string) error {
	isCached, err := e.imageCache.IsImageCached(ctx, e.client, imageName)
	if err != nil {
		e.logger.Warnf("Failed to check image cache for %s: %v", imageName, err)
	} else if isCached {
		e.logger.Debugf("Using cached image: %s", imageName)
		return nil
	}

	e.logger.Debugf("Pulling image: %s", imageName)

	pullOptions := image.PullOptions{}
	reader, err := e.client.ImagePull(ctx, imageName, pullOptions)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		e.logger.Debugf("Pull progress: %s", scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := e.imageCache.AddImage(ctx, e.client, imageName); err != nil {
		e.logger.Warnf("Failed to cache image %s: %v", imageName, err)
	}

	return nil
}

func (e *ContainerExecutor) getContainerLogs(ctx context.Context, containerID string) (string, error) {
	options := container.LogsOptions{
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
