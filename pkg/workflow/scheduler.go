package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type DependencyScheduler struct {
	jobs      map[string]*Job
	completed map[string]bool
	failed    map[string]bool
	skipped   map[string]bool
	running   map[string]bool
	logger    *logrus.Logger
	mutex     sync.RWMutex
}

type JobExecutor interface {
	ExecuteJob(ctx context.Context, job *Job, workflowVariables map[string]string) error
	InitializeVolumes(volumes map[string]VolumeSpec) error
}

func NewDependencyScheduler(jobs []Job, logger *logrus.Logger) *DependencyScheduler {
	scheduler := &DependencyScheduler{
		jobs:      make(map[string]*Job),
		completed: make(map[string]bool),
		failed:    make(map[string]bool),
		skipped:   make(map[string]bool),
		running:   make(map[string]bool),
		logger:    logger,
	}

	for i := range jobs {
		scheduler.jobs[jobs[i].Name] = &jobs[i]
	}

	return scheduler
}

func (ds *DependencyScheduler) CanExecute(jobName string) bool {
	ds.mutex.RLock()
	job := ds.jobs[jobName]
	if job == nil {
		ds.mutex.RUnlock()
		return false
	}

	if ds.completed[jobName] || ds.failed[jobName] || ds.skipped[jobName] || ds.running[jobName] {
		ds.mutex.RUnlock()
		return false
	}

	shouldSkip := false
	canExecute := true
	for _, dep := range job.DependsOn {
		if !ds.completed[dep] {
			canExecute = false
			if ds.failed[dep] || ds.skipped[dep] {
				shouldSkip = true
			}
		}
	}
	ds.mutex.RUnlock()

	if shouldSkip {
		ds.mutex.Lock()
		ds.skipped[jobName] = true
		job.Status = JobStatusSkipped
		ds.logger.Warnf("Job %s skipped due to failed dependency", jobName)
		ds.mutex.Unlock()
		return false
	}

	return canExecute
}

func (ds *DependencyScheduler) MarkRunning(jobName string) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.running[jobName] = true
}

func (ds *DependencyScheduler) MarkCompleted(jobName string) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	delete(ds.running, jobName)
	ds.completed[jobName] = true
}

func (ds *DependencyScheduler) MarkFailed(jobName string) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	delete(ds.running, jobName)
	ds.failed[jobName] = true
}

func (ds *DependencyScheduler) GetReadyJobs() []*Job {
	var readyJobs []*Job
	for jobName, job := range ds.jobs {
		if ds.CanExecute(jobName) {
			readyJobs = append(readyJobs, job)
		}
	}
	return readyJobs
}

func (ds *DependencyScheduler) HasPendingJobs() bool {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	for jobName := range ds.jobs {
		if !ds.completed[jobName] && !ds.failed[jobName] && !ds.skipped[jobName] {
			return true
		}
	}
	return false
}

func (ds *DependencyScheduler) ValidateDependencies() error {
	for jobName, job := range ds.jobs {
		for _, dep := range job.DependsOn {
			if ds.jobs[dep] == nil {
				return fmt.Errorf("job %s depends on non-existent job %s", jobName, dep)
			}
		}
	}

	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(jobName string) bool {
		visited[jobName] = true
		recursionStack[jobName] = true

		job := ds.jobs[jobName]
		for _, dep := range job.DependsOn {
			if !visited[dep] {
				if hasCycle(dep) {
					return true
				}
			} else if recursionStack[dep] {
				return true
			}
		}

		recursionStack[jobName] = false
		return false
	}

	for jobName := range ds.jobs {
		if !visited[jobName] {
			if hasCycle(jobName) {
				return fmt.Errorf("circular dependency detected involving job %s", jobName)
			}
		}
	}

	return nil
}

func (e *Engine) executeJobsWithDependencies(ctx context.Context, scheduler *DependencyScheduler, executor JobExecutor, workflowVariables map[string]string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(scheduler.jobs))

	for scheduler.HasPendingJobs() {
		readyJobs := scheduler.GetReadyJobs()

		if len(readyJobs) == 0 {
			e.logger.Debugf("No jobs ready, waiting...")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		e.logger.Debugf("Found %d ready jobs to execute", len(readyJobs))
		for _, job := range readyJobs {
			wg.Add(1)
			scheduler.MarkRunning(job.Name)

			go func(j *Job) {
				defer wg.Done()

				e.logger.Infof("Executing job: %s", j.Name)
				e.logger.Debugf("Job %s has %d dependencies", j.Name, len(j.DependsOn))

				if err := executor.ExecuteJob(ctx, j, workflowVariables); err != nil {
					e.logger.Errorf("Job %s failed: %v", j.Name, err)
					scheduler.MarkFailed(j.Name)
					errChan <- fmt.Errorf("job %s failed: %w", j.Name, err)
				} else {
					e.logger.Infof("Job %s completed successfully", j.Name)
					scheduler.MarkCompleted(j.Name)
				}
			}(job)
		}

		e.logger.Debugf("Waiting for %d jobs to complete", len(readyJobs))
		wg.Wait()

		select {
		case err := <-errChan:
			return err
		default:
		}
	}

	e.logger.Debugf("All jobs completed")
	return nil
}