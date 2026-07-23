package watchdog

import (
	"context"
	"time"
)

// Daemon runs the RepairPipeline periodically.
type Daemon struct {
	pipeline *RepairPipeline
	interval time.Duration
	cancel   context.CancelFunc
}

func NewDaemon(pipeline *RepairPipeline, interval time.Duration) *Daemon {
	return &Daemon{
		pipeline: pipeline,
		interval: interval,
	}
}

func (d *Daemon) Start(ctx context.Context, fileID string) {
	ctx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	go func() {
		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = d.pipeline.RunOnce(ctx, fileID)
			}
		}
	}()
}

func (d *Daemon) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}
