package workflow

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

type Condition interface {
	Evaluate(variables map[string]string, jobResults map[string]JobResult, logger *logrus.Logger) (bool, error)
	String() string
}

type JobResult struct {
	Status   JobStatus
	ExitCode *int
	Failed   bool
}

type AlwaysCondition struct{}
type NeverCondition struct{}

type VariableCondition struct {
	Variable string
	Equals   string
}

type JobStatusCondition struct {
	Job    string
	Status string
}

func (c AlwaysCondition) Evaluate(variables map[string]string, jobResults map[string]JobResult, logger *logrus.Logger) (bool, error) {
	logger.Debugf("Condition: always - evaluating to true")
	return true, nil
}

func (c AlwaysCondition) String() string {
	return "always"
}

func (c NeverCondition) Evaluate(variables map[string]string, jobResults map[string]JobResult, logger *logrus.Logger) (bool, error) {
	logger.Debugf("Condition: never - evaluating to false")
	return false, nil
}

func (c NeverCondition) String() string {
	return "never"
}

func (c VariableCondition) Evaluate(variables map[string]string, jobResults map[string]JobResult, logger *logrus.Logger) (bool, error) {
	value, exists := variables[c.Variable]
	if !exists {
		logger.Debugf("Condition: variable %s does not exist - evaluating to false", c.Variable)
		return false, nil
	}

	result := value == c.Equals
	logger.Debugf("Condition: variable %s = '%s' equals '%s' - evaluating to %v", c.Variable, value, c.Equals, result)
	return result, nil
}

func (c VariableCondition) String() string {
	return fmt.Sprintf("variable %s equals '%s'", c.Variable, c.Equals)
}

func (c JobStatusCondition) Evaluate(variables map[string]string, jobResults map[string]JobResult, logger *logrus.Logger) (bool, error) {
	result, exists := jobResults[c.Job]
	if !exists {
		logger.Debugf("Condition: job %s result does not exist - evaluating to false", c.Job)
		return false, nil
	}

	var matches bool
	switch strings.ToLower(c.Status) {
	case "success", "completed":
		matches = result.Status == JobStatusCompleted && !result.Failed
	case "failure", "failed":
		matches = result.Status == JobStatusFailed || result.Failed
	case "skipped":
		matches = result.Status == JobStatusSkipped
	default:
		return false, fmt.Errorf("unknown job status condition: %s", c.Status)
	}

	logger.Debugf("Condition: job %s status %s - evaluating to %v", c.Job, c.Status, matches)
	return matches, nil
}

func (c JobStatusCondition) String() string {
	return fmt.Sprintf("job %s status %s", c.Job, c.Status)
}

type ConditionEvaluator struct {
	logger *logrus.Logger
}

func NewConditionEvaluator(logger *logrus.Logger) *ConditionEvaluator {
	return &ConditionEvaluator{logger: logger}
}

func (ce *ConditionEvaluator) ShouldExecuteJob(job *Job, variables map[string]string, jobResults map[string]JobResult) (bool, error) {
	if job.When == nil {
		ce.logger.Debugf("Job %s has no condition - will execute", job.Name)
		return true, nil
	}

	result, err := job.When.Evaluate(variables, jobResults, ce.logger)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate condition for job %s: %w", job.Name, err)
	}

	if result {
		ce.logger.Infof("Job %s condition (%s) evaluated to true - will execute", job.Name, job.When.String())
	} else {
		ce.logger.Infof("Job %s condition (%s) evaluated to false - will skip", job.Name, job.When.String())
	}

	return result, nil
}
