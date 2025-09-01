package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RunnerRegistry manages registered runners
type RunnerRegistry struct {
	runners          map[string]*Runner
	mutex            sync.RWMutex
	logger           *logrus.Logger
	heartbeatTimeout time.Duration
}

// NewRunnerRegistry creates a new runner registry
func NewRunnerRegistry(logger *logrus.Logger) *RunnerRegistry {
	registry := &RunnerRegistry{
		runners:          make(map[string]*Runner),
		logger:           logger,
		heartbeatTimeout: 30 * time.Second, // 30 seconds timeout
	}

	// Start heartbeat monitor
	go registry.monitorHeartbeats()

	return registry
}

// RegisterRunner registers a new runner
func (rr *RunnerRegistry) RegisterRunner(reg *RunnerRegistration) (*Runner, error) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	now := time.Now()
	
	// Check if runner already exists
	if existingRunner, exists := rr.runners[reg.ID]; exists {
		// Update existing runner
		existingRunner.Name = reg.Name
		existingRunner.Version = reg.Version
		existingRunner.Capabilities = reg.Capabilities
		existingRunner.Tags = reg.Tags
		existingRunner.Metadata = reg.Metadata
		existingRunner.Status = RunnerStatusOnline
		existingRunner.LastHeartbeat = now
		
		rr.logger.Infof("Re-registered runner %s (%s)", reg.ID, reg.Name)
		return existingRunner, nil
	}

	// Create new runner
	runner := &Runner{
		ID:            reg.ID,
		Name:          reg.Name,
		Version:       reg.Version,
		Status:        RunnerStatusOnline,
		LastHeartbeat: now,
		RegisteredAt:  now,
		Capabilities:  reg.Capabilities,
		Tags:          reg.Tags,
		Metadata:      reg.Metadata,
	}

	rr.runners[reg.ID] = runner
	rr.logger.Infof("Registered new runner %s (%s)", reg.ID, reg.Name)
	
	return runner, nil
}

// UpdateHeartbeat updates the last heartbeat for a runner
func (rr *RunnerRegistry) UpdateHeartbeat(runnerID string, status RunnerStatus, currentJobID string) error {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	runner, exists := rr.runners[runnerID]
	if !exists {
		return fmt.Errorf("runner %s not registered", runnerID)
	}

	runner.LastHeartbeat = time.Now()
	runner.Status = status
	runner.CurrentJobID = currentJobID

	rr.logger.Debugf("Updated heartbeat for runner %s (status: %s)", runnerID, status)
	return nil
}

// GetRunner retrieves a runner by ID
func (rr *RunnerRegistry) GetRunner(runnerID string) (*Runner, error) {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	runner, exists := rr.runners[runnerID]
	if !exists {
		return nil, fmt.Errorf("runner %s not found", runnerID)
	}

	return runner, nil
}

// GetAvailableRunners returns runners that are online and not busy
func (rr *RunnerRegistry) GetAvailableRunners() []*Runner {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	var available []*Runner
	for _, runner := range rr.runners {
		if runner.Status == RunnerStatusOnline {
			available = append(available, runner)
		}
	}

	return available
}

// ListRunners returns all runners with pagination
func (rr *RunnerRegistry) ListRunners(page, pageSize int) ([]*Runner, int, error) {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	// Convert map to slice for pagination
	allRunners := make([]*Runner, 0, len(rr.runners))
	for _, runner := range rr.runners {
		allRunners = append(allRunners, runner)
	}

	// Simple pagination
	total := len(allRunners)
	start := (page - 1) * pageSize
	if start >= total {
		return []*Runner{}, total, nil
	}
	
	end := start + pageSize
	if end > total {
		end = total
	}

	return allRunners[start:end], total, nil
}

// UnregisterRunner removes a runner from the registry
func (rr *RunnerRegistry) UnregisterRunner(runnerID string) error {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if _, exists := rr.runners[runnerID]; !exists {
		return fmt.Errorf("runner %s not found", runnerID)
	}

	delete(rr.runners, runnerID)
	rr.logger.Infof("Unregistered runner %s", runnerID)
	
	return nil
}

// GetStats returns registry statistics
func (rr *RunnerRegistry) GetStats() map[string]int {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	stats := map[string]int{
		"total":    len(rr.runners),
		"online":   0,
		"offline":  0,
		"busy":     0,
		"disabled": 0,
	}

	for _, runner := range rr.runners {
		switch runner.Status {
		case RunnerStatusOnline:
			stats["online"]++
		case RunnerStatusOffline:
			stats["offline"]++
		case RunnerStatusBusy:
			stats["busy"]++
		case RunnerStatusDisabled:
			stats["disabled"]++
		}
	}

	return stats
}

// monitorHeartbeats runs in the background and marks runners as offline if they miss heartbeats
func (rr *RunnerRegistry) monitorHeartbeats() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rr.checkHeartbeats()
		}
	}
}

// checkHeartbeats checks for missed heartbeats and marks runners as offline
func (rr *RunnerRegistry) checkHeartbeats() {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	now := time.Now()
	
	for _, runner := range rr.runners {
		if runner.Status == RunnerStatusOnline || runner.Status == RunnerStatusBusy {
			if now.Sub(runner.LastHeartbeat) > rr.heartbeatTimeout {
				runner.Status = RunnerStatusOffline
				runner.CurrentJobID = "" // Clear current job
				rr.logger.Warnf("Runner %s marked as offline (missed heartbeat)", runner.ID)
			}
		}
	}
}

// FindBestRunner finds the best available runner for a job (simple round-robin for now)
func (rr *RunnerRegistry) FindBestRunner(job *Job) (*Runner, error) {
	availableRunners := rr.GetAvailableRunners()
	
	if len(availableRunners) == 0 {
		return nil, fmt.Errorf("no available runners")
	}

	// Simple selection: first available runner
	// TODO: Implement more sophisticated matching based on tags, capabilities, load, etc.
	return availableRunners[0], nil
}