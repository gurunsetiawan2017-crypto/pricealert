package scheduler

import (
	"context"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/runtime/state"
	"github.com/pricealert/pricealert/internal/runtime/worker"
)

type KeywordSource interface {
	ListKeywords(context.Context) ([]domain.TrackedKeyword, error)
}

type Clock interface {
	Now() time.Time
}

type Scheduler struct {
	source KeywordSource
	state  *state.Store
	worker *worker.Worker
	clock  Clock
}

type RunResult struct {
	Started         []string
	Skipped         []string
	CapacityBlocked []string
	RunningCount    int
	MaxConcurrent   int
}

func New(source KeywordSource, stateStore *state.Store, worker *worker.Worker, clock Clock) *Scheduler {
	return &Scheduler{
		source: source,
		state:  stateStore,
		worker: worker,
		clock:  clock,
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) (RunResult, error) {
	keywords, err := s.source.ListKeywords(ctx)
	if err != nil {
		return RunResult{}, err
	}

	s.state.EnsureKeywords(keywords)
	now := s.clock.Now()

	result := RunResult{
		Started:         make([]string, 0),
		Skipped:         make([]string, 0),
		CapacityBlocked: make([]string, 0),
	}

	for _, keyword := range keywords {
		if keyword.Status != domain.TrackedKeywordStatusActive {
			result.Skipped = append(result.Skipped, keyword.ID)
			continue
		}

		if !s.state.IsEligible(keyword.ID, now) {
			result.Skipped = append(result.Skipped, keyword.ID)
			continue
		}

		if !s.worker.Start(ctx, keyword) {
			if s.state.Snapshot(keyword.ID).Running {
				result.Skipped = append(result.Skipped, keyword.ID)
			} else {
				result.CapacityBlocked = append(result.CapacityBlocked, keyword.ID)
			}
			continue
		}

		result.Started = append(result.Started, keyword.ID)
	}

	workerStatus := s.worker.Status()
	result.RunningCount = workerStatus.RunningCount
	result.MaxConcurrent = workerStatus.MaxConcurrent

	return result, nil
}
