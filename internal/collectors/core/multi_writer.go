package core

import (
	"context"

	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
)

type MultiWriter []ResultWriter

func (w MultiWriter) Write(ctx context.Context, dataType string, result normalizers.NormalizedResult, job scheduler.Job) error {
	for _, writer := range w {
		if writer == nil {
			continue
		}
		if err := writer.Write(ctx, dataType, result, job); err != nil {
			return err
		}
	}
	return nil
}
