package core

import (
	"context"

	"aggregator-services/internal/scheduler"
)

type DataTypeExecutor map[string]scheduler.Executor

func (r DataTypeExecutor) Execute(ctx context.Context, job scheduler.Job) error {
	if executor, ok := r[job.DataType]; ok {
		return executor.Execute(ctx, job)
	}
	if executor, ok := r["*"]; ok {
		return executor.Execute(ctx, job)
	}
	return scheduler.NewExecutionError("parse", false, ErrNoExecutor)
}
