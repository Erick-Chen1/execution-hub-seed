package orchestrator

import (
	"context"
	"encoding/json"
	"time"

	"github.com/execution-hub/execution-hub/internal/domain/task"
)

// DefaultAgentRunner is a simple stub runner for demos.
type DefaultAgentRunner struct{}

func (r *DefaultAgentRunner) Run(ctx context.Context, t *task.Task, step *task.Step) (json.RawMessage, error) {
	_ = ctx
	result := map[string]interface{}{
		"taskId":   t.TaskID.String(),
		"stepId":   step.StepID.String(),
		"stepKey":  step.StepKey,
		"executor": step.ExecutorRef,
		"output":   "agent output placeholder",
		"time":     time.Now().UTC().Format(time.RFC3339),
	}
	return json.Marshal(result)
}
