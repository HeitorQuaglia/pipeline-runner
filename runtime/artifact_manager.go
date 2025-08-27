package runtime

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"

	"basic-container-runtime/pkg/workflow"
)

type ArtifactManager struct {
	artifacts map[string]string
	baseDir   string
	logger    *logrus.Logger
	mutex     sync.RWMutex
}

func NewArtifactManager(logger *logrus.Logger) (*ArtifactManager, error) {
	baseDir := ".runner-artifacts"

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for artifacts directory: %w", err)
	}

	if err := os.MkdirAll(absBaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	return &ArtifactManager{
		artifacts: make(map[string]string),
		baseDir:   absBaseDir,
		logger:    logger,
	}, nil
}

func (am *ArtifactManager) StoreArtifacts(containerID string, artifacts []workflow.ArtifactSpec) error {
	if len(artifacts) == 0 {
		return nil
	}

	am.logger.Infof("Storing %d artifacts from container %s", len(artifacts), containerID[:12])

	for _, artifact := range artifacts {
		if err := am.storeArtifact(containerID, artifact); err != nil {
			return fmt.Errorf("failed to store artifact %s: %w", artifact.Name, err)
		}
	}

	return nil
}

func (am *ArtifactManager) storeArtifact(containerID string, artifact workflow.ArtifactSpec) error {
	am.logger.Debugf("Storing artifact %s from container %s:%s", artifact.Name, containerID[:12], artifact.Path)

	artifactDir := filepath.Join(am.baseDir, artifact.Name)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory %s: %w", artifactDir, err)
	}

	placeholderPath := filepath.Join(artifactDir, ".artifact_info")
	placeholderContent := fmt.Sprintf("Artifact: %s\nContainer: %s\nSource Path: %s\n",
		artifact.Name, containerID, artifact.Path)

	if err := os.WriteFile(placeholderPath, []byte(placeholderContent), 0644); err != nil {
		return fmt.Errorf("failed to create artifact placeholder: %w", err)
	}

	am.mutex.Lock()
	am.artifacts[artifact.Name] = artifactDir
	am.mutex.Unlock()

	am.logger.Infof("Artifact %s stored at %s", artifact.Name, artifactDir)
	return nil
}

func (am *ArtifactManager) CopyFromContainer(ctx context.Context, dockerClient *client.Client, containerID string, artifact workflow.ArtifactSpec) error {
	artifactDir := filepath.Join(am.baseDir, artifact.Name)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory %s: %w", artifactDir, err)
	}

	am.logger.Debugf("Copying from container %s:%s to %s", containerID[:12], artifact.Path, artifactDir)

	reader, stat, err := dockerClient.CopyFromContainer(ctx, containerID, artifact.Path)
	if err != nil {
		return fmt.Errorf("failed to copy from container %s:%s: %w", containerID[:12], artifact.Path, err)
	}
	defer reader.Close()

	if err := am.extractTarToDirectory(reader, artifactDir, stat.Name); err != nil {
		return fmt.Errorf("failed to extract artifact to %s: %w", artifactDir, err)
	}

	am.mutex.Lock()
	am.artifacts[artifact.Name] = artifactDir
	am.mutex.Unlock()

	am.logger.Infof("Successfully copied artifact %s from container %s:%s to %s",
		artifact.Name, containerID[:12], artifact.Path, artifactDir)

	return nil
}

func (am *ArtifactManager) extractTarToDirectory(reader io.Reader, destDir string, baseName string) error {
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		relativePath := header.Name
		if baseName != "" && header.Name == baseName && header.Typeflag == tar.TypeDir {
			continue
		}
		if baseName != "" && filepath.HasPrefix(header.Name, baseName+"/") {
			relativePath = header.Name[len(baseName)+1:]
		}

		if relativePath == "" {
			continue
		}

		destPath := filepath.Join(destDir, relativePath)

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(destPath), err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
			am.logger.Debugf("Created directory: %s", destPath)

		case tar.TypeReg:
			file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", destPath, err)
			}

			_, err = io.Copy(file, tarReader)
			file.Close()
			if err != nil {
				return fmt.Errorf("failed to write file %s: %w", destPath, err)
			}
			am.logger.Debugf("Extracted file: %s (%d bytes)", destPath, header.Size)

		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, destPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", destPath, err)
			}
			am.logger.Debugf("Created symlink: %s -> %s", destPath, header.Linkname)

		default:
			am.logger.Debugf("Skipping unsupported file type: %c for %s", header.Typeflag, destPath)
		}
	}

	return nil
}

func (am *ArtifactManager) MountArtifacts(artifactMounts []workflow.ArtifactMount) ([]string, error) {
	var mountCommands []string

	if len(artifactMounts) == 0 {
		return mountCommands, nil
	}

	am.logger.Debugf("Preparing to mount %d artifacts", len(artifactMounts))

	for _, mount := range artifactMounts {
		artifactPath, exists := am.GetArtifactPath(mount.Name)
		if !exists {
			return nil, fmt.Errorf("artifact %s not found", mount.Name)
		}

		am.logger.Debugf("Will mount artifact %s: %s -> %s", mount.Name, artifactPath, mount.Path)

		sourceInfo, err := os.Stat(artifactPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat artifact %s at %s: %w", mount.Name, artifactPath, err)
		}

		if sourceInfo.IsDir() {
			mountCommands = append(mountCommands, fmt.Sprintf("-v %s:%s:ro", artifactPath, mount.Path))
		} else {
			mountCommands = append(mountCommands, fmt.Sprintf("-v %s:%s:ro", artifactPath, mount.Path))
		}

	}

	return mountCommands, nil
}

func (am *ArtifactManager) GetArtifactPath(artifactName string) (string, bool) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	path, exists := am.artifacts[artifactName]
	return path, exists
}

func (am *ArtifactManager) ListArtifacts() []string {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	var artifactNames []string
	for name := range am.artifacts {
		artifactNames = append(artifactNames, name)
	}
	return artifactNames
}

func (am *ArtifactManager) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (am *ArtifactManager) CopyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return am.CopyFile(path, destPath)
	})
}

func (am *ArtifactManager) Cleanup() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.logger.Debugf("Cleaning up artifacts directory: %s", am.baseDir)

	if err := os.RemoveAll(am.baseDir); err != nil {
		am.logger.Warnf("Failed to remove artifacts directory %s: %v", am.baseDir, err)
		return err
	}

	am.artifacts = make(map[string]string)
	am.logger.Infof("Artifacts cleanup completed")
	return nil
}
