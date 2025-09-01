package runtime

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	
	"basic-container-runtime/pkg/secrets"
	"basic-container-runtime/pkg/workflow"
)

// Executor is an alias for ContainerExecutor for backwards compatibility
type Executor = ContainerExecutor

// NewExecutor creates a new executor instance
func NewExecutor(logger *logrus.Logger) *Executor {
	executor, err := NewContainerExecutor()
	if err != nil {
		logger.Fatalf("Failed to create executor: %v", err)
	}
	
	// Override the logger with the provided one
	executor.logger = logger
	return executor
}

// ExecutionContext contains context for workflow execution
type ExecutionContext struct {
	WorkflowID string
	Variables  map[string]string
	Secrets    map[string]string
	Logger     *logrus.Logger
}

// ExecutionResult represents the result of workflow execution
type ExecutionResult struct {
	Success      bool
	ErrorMessage string
	ExitCode     int
}

// LogCollector interface for collecting execution logs
type LogCollector interface {
	CollectLog(timestamp time.Time, level, message, source string)
	Flush()
}

// DefaultLogCollector is a basic log collector that does nothing
type DefaultLogCollector struct{}

func (c *DefaultLogCollector) CollectLog(timestamp time.Time, level, message, source string) {}
func (c *DefaultLogCollector) Flush() {}

// ExecuteWorkflow executes a workflow with default log collector
func (e *Executor) ExecuteWorkflow(wf *workflow.Workflow, ctx *ExecutionContext) (*ExecutionResult, error) {
	return e.ExecuteWorkflowWithCollector(wf, ctx, &DefaultLogCollector{})
}

// ExecuteWorkflowWithCollector executes a workflow with a custom log collector
func (e *Executor) ExecuteWorkflowWithCollector(wf *workflow.Workflow, ctx *ExecutionContext, collector LogCollector) (*ExecutionResult, error) {
	e.logger.Infof("Starting workflow execution: %s (%s)", wf.ID, wf.Name)
	
	defer collector.Flush()
	
	// Set up secret manager if secrets are provided
	if ctx.Secrets != nil && len(ctx.Secrets) > 0 {
		secretManager := secrets.NewSecretManager(e.logger)
		for name, value := range ctx.Secrets {
			secretManager.SetSecret(name, value)
		}
		e.SetSecretManager(secretManager)
	}
	
	// Execute all jobs in dependency order
	jobQueue := NewJobQueue(wf.Jobs)
	
	for jobQueue.Len() > 0 {
		// Find jobs that can be executed (all dependencies met)
		readyJobs := jobQueue.GetReadyJobs()
		
		if len(readyJobs) == 0 {
			return &ExecutionResult{
				Success:      false,
				ErrorMessage: "Circular dependency detected or unresolved dependencies",
				ExitCode:     1,
			}, nil
		}
		
		// Execute ready jobs
		for _, job := range readyJobs {
			collector.CollectLog(time.Now(), "info", fmt.Sprintf("Starting job: %s", job.Name), job.Name)
			
			// Check job condition
			if job.When != nil {
				// Convert CompletedJobs map to JobResult format
				jobResults := make(map[string]workflow.JobResult)
				for jobName, success := range jobQueue.CompletedJobs {
					status := workflow.JobStatusCompleted
					if !success {
						status = workflow.JobStatusFailed
					}
					jobResults[jobName] = workflow.JobResult{
						Status: status,
						Failed: !success,
					}
				}
				
				shouldRun, err := job.When.Evaluate(ctx.Variables, jobResults, e.logger)
				if err != nil {
					collector.CollectLog(time.Now(), "error", fmt.Sprintf("Condition evaluation failed: %v", err), job.Name)
					return &ExecutionResult{
						Success:      false,
						ErrorMessage: fmt.Sprintf("Condition evaluation failed for job %s: %v", job.Name, err),
						ExitCode:     1,
					}, nil
				}
				
				if !shouldRun {
					collector.CollectLog(time.Now(), "info", "Job skipped due to condition", job.Name)
					job.Status = workflow.JobStatusSkipped
					jobQueue.MarkJobCompleted(job.Name, true)
					continue
				}
			}
			
			// Execute the job
			err := e.ExecuteJob(nil, job, ctx.Variables) // nil context for now, could be improved
			
			if err != nil {
				collector.CollectLog(time.Now(), "error", fmt.Sprintf("Job failed: %v", err), job.Name)
				
				if !job.AllowFailure {
					return &ExecutionResult{
						Success:      false,
						ErrorMessage: fmt.Sprintf("Job %s failed: %v", job.Name, err),
						ExitCode:     1,
					}, nil
				} else {
					collector.CollectLog(time.Now(), "warn", "Job failed but marked as allowed to fail", job.Name)
				}
			}
			
			collector.CollectLog(time.Now(), "info", fmt.Sprintf("Job completed: %s", job.Name), job.Name)
			jobQueue.MarkJobCompleted(job.Name, err == nil)
		}
	}
	
	e.logger.Infof("Workflow execution completed: %s", wf.ID)
	return &ExecutionResult{
		Success:      true,
		ErrorMessage: "",
		ExitCode:     0,
	}, nil
}

// JobQueue manages job execution queue with dependencies
type JobQueue struct {
	jobs          []*workflow.Job
	CompletedJobs map[string]bool // job name -> success
}

// NewJobQueue creates a new job queue
func NewJobQueue(jobs []workflow.Job) *JobQueue {
	jobPointers := make([]*workflow.Job, len(jobs))
	for i := range jobs {
		jobPointers[i] = &jobs[i]
	}
	
	return &JobQueue{
		jobs:          jobPointers,
		CompletedJobs: make(map[string]bool),
	}
}

// GetReadyJobs returns jobs that can be executed (all dependencies met)
func (jq *JobQueue) GetReadyJobs() []*workflow.Job {
	var ready []*workflow.Job
	
	for _, job := range jq.jobs {
		if job.Status != workflow.JobStatusPending {
			continue
		}
		
		// Check if all dependencies are met
		canRun := true
		for _, dep := range job.DependsOn {
			if success, completed := jq.CompletedJobs[dep]; !completed || !success {
				canRun = false
				break
			}
		}
		
		if canRun {
			ready = append(ready, job)
		}
	}
	
	return ready
}

// MarkJobCompleted marks a job as completed
func (jq *JobQueue) MarkJobCompleted(jobName string, success bool) {
	jq.CompletedJobs[jobName] = success
	
	// Remove from pending jobs
	for i, job := range jq.jobs {
		if job.Name == jobName {
			jq.jobs = append(jq.jobs[:i], jq.jobs[i+1:]...)
			break
		}
	}
}

// Len returns the number of pending jobs
func (jq *JobQueue) Len() int {
	return len(jq.jobs)
}