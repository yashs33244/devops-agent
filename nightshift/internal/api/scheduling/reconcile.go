package scheduling

import (
	"context"
	"errors"
	"fmt"
)

// ReconcileResult is what the reconciler returns at startup. Counts
// are best-effort: a partial-failure run still returns the counts
// for what did succeed, plus an aggregated error for what didn't.
type ReconcileResult struct {
	Applied int // CronJobs created or re-applied for live Records
	Reaped  int // CronJobs deleted because their Record is gone
	Errors  []error
}

// ReconcileSchedules walks the `schedules` Record collection +
// scheduler.List() and forces them into agreement. Mirrors cr0n's
// scheduler.sync() and matches the chunk-11 ReconcileCatalog pattern.
//
// Called once from main.go after Service construction. Drift between
// reconcile passes (someone kubectl-deletes a CronJob) is observable
// only on the next API restart — a periodic reconciler is a future
// chunk if operators need it.
func (s *Service) ReconcileSchedules(ctx context.Context) (ReconcileResult, error) {
	result := ReconcileResult{}

	recs, err := s.listAllSchedules(ctx)
	if err != nil {
		return result, fmt.Errorf("scheduling: list records: %w", err)
	}
	managed, err := s.scheduler.List(ctx)
	if err != nil {
		return result, fmt.Errorf("scheduling: list scheduler: %w", err)
	}

	recordByID := make(map[string]bool, len(recs))
	for _, sch := range recs {
		recordByID[sch.GetId()] = true
	}
	managedByID := make(map[string]bool, len(managed))
	for _, m := range managed {
		managedByID[m.ID] = true
	}

	// 1. Apply for every Record. Idempotent — `Apply` is create-or-
	// update; existing CronJobs absorb spec drift on restart.
	for _, sch := range recs {
		if err := s.scheduler.Apply(ctx, s.specFor(sch)); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("apply %s: %w", sch.GetId(), err))
			s.logger.Warn("scheduling: reconcile apply failed", "schedule_id", sch.GetId(), "err", err)
			continue
		}
		result.Applied++
	}

	// 2. Reap orphans — CronJob exists, Record is gone (deleted out-
	// of-band, or the API was killed mid-DeleteSchedule and only the
	// scheduler.Delete or only the records.Delete succeeded).
	for id := range managedByID {
		if recordByID[id] {
			continue
		}
		if err := s.scheduler.Delete(ctx, id); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("reap %s: %w", id, err))
			s.logger.Warn("scheduling: reconcile reap failed", "schedule_id", id, "err", err)
			continue
		}
		result.Reaped++
	}

	if len(result.Errors) > 0 {
		return result, errors.Join(result.Errors...)
	}
	return result, nil
}
