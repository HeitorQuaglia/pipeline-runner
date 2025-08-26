package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"

	"basic-container-runtime/pkg/workflow"
)

type VolumeManager struct {
	volumes map[string]string
	baseDir string
	logger  *logrus.Logger
	mutex   sync.RWMutex
}

func NewVolumeManager(logger *logrus.Logger) (*VolumeManager, error) {
	baseDir := ".runner-volumes"

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for volumes directory: %w", err)
	}

	if err := os.MkdirAll(absBaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create volumes directory: %w", err)
	}

	return &VolumeManager{
		volumes: make(map[string]string),
		baseDir: absBaseDir,
		logger:  logger,
	}, nil
}

func (vm *VolumeManager) EnsureVolume(volumeName string) (string, error) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	if hostPath, exists := vm.volumes[volumeName]; exists {
		return hostPath, nil
	}

	hostPath := filepath.Join(vm.baseDir, volumeName)
	if err := os.MkdirAll(hostPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create volume directory %s: %w", hostPath, err)
	}

	vm.volumes[volumeName] = hostPath
	vm.logger.Debugf("Created volume %s at %s", volumeName, hostPath)
	return hostPath, nil
}

func (vm *VolumeManager) GetVolumePath(volumeName string) (string, bool) {
	vm.mutex.RLock()
	defer vm.mutex.RUnlock()

	hostPath, exists := vm.volumes[volumeName]
	return hostPath, exists
}

func (vm *VolumeManager) Cleanup() error {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()

	for volumeName, hostPath := range vm.volumes {
		vm.logger.Debugf("Cleaning up volume %s at %s", volumeName, hostPath)
		if err := os.RemoveAll(hostPath); err != nil {
			vm.logger.Warnf("Failed to remove volume directory %s: %v", hostPath, err)
		}
	}

	vm.volumes = make(map[string]string)
	return nil
}

func (e *ContainerExecutor) InitializeVolumes(volumes map[string]workflow.VolumeSpec) error {
	if volumes == nil {
		return nil
	}

	e.logger.Debugf("Initializing %d volumes", len(volumes))
	for volumeName, volumeSpec := range volumes {
		switch volumeSpec.Type {
		case "host":
			if volumeSpec.HostPath != "" {
				absHostPath, err := filepath.Abs(volumeSpec.HostPath)
				if err != nil {
					return fmt.Errorf("failed to get absolute path for %s: %w", volumeSpec.HostPath, err)
				}
				if err := os.MkdirAll(absHostPath, 0755); err != nil {
					return fmt.Errorf("failed to create host path %s for volume %s: %w", absHostPath, volumeName, err)
				}
				e.volumeManager.volumes[volumeName] = absHostPath
			} else {
				if _, err := e.volumeManager.EnsureVolume(volumeName); err != nil {
					return fmt.Errorf("failed to ensure volume %s: %w", volumeName, err)
				}
			}
		default:
			return fmt.Errorf("unsupported volume type: %s for volume %s", volumeSpec.Type, volumeName)
		}
	}

	return nil
}