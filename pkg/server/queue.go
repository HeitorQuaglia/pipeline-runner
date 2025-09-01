package server

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// JobQueue manages the queue of pending jobs
type JobQueue struct {
	jobs    map[string]*Job
	pending *PriorityQueue
	running map[string]*Job
	mutex   sync.RWMutex
	logger  *logrus.Logger
}

// NewJobQueue creates a new job queue
func NewJobQueue(logger *logrus.Logger) *JobQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	
	return &JobQueue{
		jobs:    make(map[string]*Job),
		pending: &pq,
		running: make(map[string]*Job),
		logger:  logger,
	}
}

// AddJob adds a new job to the queue
func (jq *JobQueue) AddJob(submission *WorkflowSubmission) (*Job, error) {
	jq.mutex.Lock()
	defer jq.mutex.Unlock()

	job := &Job{
		ID:          generateJobID(),
		WorkflowID:  generateWorkflowID(),
		Name:        submission.Name,
		Description: submission.Description,
		YAML:        submission.YAML,
		Variables:   submission.Variables,
		Secrets:     submission.Secrets,
		Status:      JobStatusPending,
		CreatedAt:   time.Now(),
		Priority:    0, // Default priority
		Logs:        make([]LogEntry, 0),
	}

	// Store job
	jq.jobs[job.ID] = job
	
	// Add to pending queue
	heap.Push(jq.pending, job)
	
	jq.logger.Infof("Added job %s (%s) to queue", job.ID, job.Name)
	return job, nil
}

// GetNextJob gets the next job for a runner
func (jq *JobQueue) GetNextJob(runnerID string) (*Job, error) {
	jq.mutex.Lock()
	defer jq.mutex.Unlock()

	if jq.pending.Len() == 0 {
		return nil, fmt.Errorf("no jobs available")
	}

	// Get highest priority job
	job := heap.Pop(jq.pending).(*Job)
	
	// Mark as running
	job.Status = JobStatusRunning
	job.RunnerID = runnerID
	now := time.Now()
	job.StartedAt = &now
	
	// Move to running map
	jq.running[job.ID] = job
	
	jq.logger.Infof("Assigned job %s to runner %s", job.ID, runnerID)
	return job, nil
}

// CompleteJob marks a job as completed
func (jq *JobQueue) CompleteJob(jobID string, result *JobResult) error {
	jq.mutex.Lock()
	defer jq.mutex.Unlock()

	job, exists := jq.running[jobID]
	if !exists {
		return fmt.Errorf("job %s not found in running jobs", jobID)
	}

	// Update job status
	now := time.Now()
	job.CompletedAt = &now
	job.Result = result
	
	if result.Success {
		job.Status = JobStatusCompleted
		jq.logger.Infof("Job %s completed successfully", jobID)
	} else {
		job.Status = JobStatusFailed
		jq.logger.Warnf("Job %s failed: %s", jobID, result.Error)
	}

	// Remove from running
	delete(jq.running, jobID)
	
	return nil
}

// FailJob marks a job as failed
func (jq *JobQueue) FailJob(jobID string, errorMsg string) error {
	jq.mutex.Lock()
	defer jq.mutex.Unlock()

	job, exists := jq.running[jobID]
	if !exists {
		return fmt.Errorf("job %s not found in running jobs", jobID)
	}

	// Update job status
	now := time.Now()
	job.CompletedAt = &now
	job.Status = JobStatusFailed
	job.Result = &JobResult{
		Success: false,
		Error:   errorMsg,
	}

	// Remove from running
	delete(jq.running, jobID)
	
	jq.logger.Errorf("Job %s failed: %s", jobID, errorMsg)
	return nil
}

// GetJob retrieves a job by ID
func (jq *JobQueue) GetJob(jobID string) (*Job, error) {
	jq.mutex.RLock()
	defer jq.mutex.RUnlock()

	job, exists := jq.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	return job, nil
}

// AddLogs adds log entries to a job
func (jq *JobQueue) AddLogs(jobID string, logs []LogEntry) error {
	jq.mutex.Lock()
	defer jq.mutex.Unlock()

	job, exists := jq.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	job.Logs = append(job.Logs, logs...)
	jq.logger.Debugf("Added %d log entries to job %s", len(logs), jobID)
	
	return nil
}

// ListJobs returns all jobs with pagination
func (jq *JobQueue) ListJobs(page, pageSize int) ([]*Job, int, error) {
	jq.mutex.RLock()
	defer jq.mutex.RUnlock()

	// Convert map to slice for sorting/pagination
	allJobs := make([]*Job, 0, len(jq.jobs))
	for _, job := range jq.jobs {
		allJobs = append(allJobs, job)
	}

	// Simple pagination
	total := len(allJobs)
	start := (page - 1) * pageSize
	if start >= total {
		return []*Job{}, total, nil
	}
	
	end := start + pageSize
	if end > total {
		end = total
	}

	return allJobs[start:end], total, nil
}

// GetStats returns queue statistics
func (jq *JobQueue) GetStats() map[string]int {
	jq.mutex.RLock()
	defer jq.mutex.RUnlock()

	stats := map[string]int{
		"total":     len(jq.jobs),
		"pending":   jq.pending.Len(),
		"running":   len(jq.running),
		"completed": 0,
		"failed":    0,
	}

	// Count completed and failed jobs
	for _, job := range jq.jobs {
		if job.Status == JobStatusCompleted {
			stats["completed"]++
		} else if job.Status == JobStatusFailed {
			stats["failed"]++
		}
	}

	return stats
}

// PriorityQueue implements a priority queue for jobs
type PriorityQueue []*Job

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// Higher priority first, then older jobs first
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*Job))
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return "job-" + uuid.New().String()[:8]
}

// generateWorkflowID generates a unique workflow ID
func generateWorkflowID() string {
	return "workflow-" + uuid.New().String()[:8]
}