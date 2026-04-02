package app

import (
	"context"

	"github.com/charmbracelet/bubbletea"

	"github.com/pricealert/pricealert/internal/dto"
	"github.com/pricealert/pricealert/internal/infra/idgen"
	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
	kwservice "github.com/pricealert/pricealert/internal/service/keyword"
	"github.com/pricealert/pricealert/internal/service/query"
	"github.com/pricealert/pricealert/internal/tui"
)

func newQueryService(repos appRepositories, runtimeStatus query.RuntimeStatusProvider) *query.Service {
	return query.NewService(
		repos.trackedKeywords,
		repos.groupedListings,
		repos.snapshots,
		repos.pricePoints,
		repos.alertEvents,
		runtimeStatus,
	)
}

func newKeywordService(repos appRepositories, defaultIntervalMins int) *kwservice.Service {
	return kwservice.NewService(
		idgen.NewULIDGenerator(),
		systemClock{},
		repos.trackedKeywords,
		defaultIntervalMins,
	)
}

func newTUIProgram(queries *query.Service, trigger runtimeTriggerAdapter, keywords keywordActionAdapter) *tea.Program {
	model := tui.NewModel(queries, trigger, keywords)
	return tea.NewProgram(model)
}

type runtimeStatusSource interface {
	RuntimeStatus() RuntimeStatus
}

type runtimeStatusAdapter struct {
	source runtimeStatusSource
}

func newRuntimeStatusAdapter(source runtimeStatusSource) runtimeStatusAdapter {
	return runtimeStatusAdapter{source: source}
}

func (a runtimeStatusAdapter) Summary(context.Context) (*dto.RuntimeStatusSummary, error) {
	status := a.source.RuntimeStatus()
	return &dto.RuntimeStatusSummary{
		AcceptingNewWork:       status.AcceptingNewWork,
		RunningCount:           status.RunningCount,
		MaxConcurrent:          status.MaxConcurrent,
		ReconciledRunningJobs:  status.ReconciledRunningJobs,
		LastReconciledAt:       status.LastReconciledAt,
		PrunedRawListings:      status.PrunedRawListings,
		LastPrunedAt:           status.LastPrunedAt,
		PrunedAlertEvents:      status.PrunedAlertEvents,
		LastAlertPrunedAt:      status.LastAlertPrunedAt,
		PrunedPricePoints:      status.PrunedPricePoints,
		LastPricePointPrunedAt: status.LastPricePointPrunedAt,
	}, nil
}

type runtimeRunner interface {
	RunRuntimeOnce(context.Context) (rtscheduler.RunResult, error)
}

type runtimeTriggerAdapter struct {
	run func(context.Context) (rtscheduler.RunResult, error)
}

func newRuntimeTrigger(app runtimeRunner) runtimeTriggerAdapter {
	return runtimeTriggerAdapter{run: app.RunRuntimeOnce}
}

func (a runtimeTriggerAdapter) RunOnce(ctx context.Context) (tui.RuntimeRunResult, error) {
	result, err := a.run(ctx)
	if err != nil {
		return tui.RuntimeRunResult{}, err
	}
	return tui.RuntimeRunResult{
		Started: result.Started,
		Skipped: result.Skipped,
	}, nil
}

type keywordActionService interface {
	AddKeyword(context.Context, string) error
	PauseKeyword(context.Context, string) error
	ResumeKeyword(context.Context, string) error
	ArchiveKeyword(context.Context, string) error
	UpdateThreshold(context.Context, string, *int64) error
	UpdateInterval(context.Context, string, int) error
	UpdateBasicFilter(context.Context, string, *string) error
	SetTelegramEnabled(context.Context, string, bool) error
}

type keywordActionAdapter struct {
	service keywordActionService
}

func newKeywordActions(service keywordActionService) keywordActionAdapter {
	return keywordActionAdapter{service: service}
}

func (a keywordActionAdapter) AddKeyword(ctx context.Context, keyword string) error {
	return a.service.AddKeyword(ctx, keyword)
}

func (a keywordActionAdapter) PauseKeyword(ctx context.Context, keywordID string) error {
	return a.service.PauseKeyword(ctx, keywordID)
}

func (a keywordActionAdapter) ResumeKeyword(ctx context.Context, keywordID string) error {
	return a.service.ResumeKeyword(ctx, keywordID)
}

func (a keywordActionAdapter) ArchiveKeyword(ctx context.Context, keywordID string) error {
	return a.service.ArchiveKeyword(ctx, keywordID)
}

func (a keywordActionAdapter) UpdateThreshold(ctx context.Context, keywordID string, threshold *int64) error {
	return a.service.UpdateThreshold(ctx, keywordID, threshold)
}

func (a keywordActionAdapter) UpdateInterval(ctx context.Context, keywordID string, intervalMinutes int) error {
	return a.service.UpdateInterval(ctx, keywordID, intervalMinutes)
}

func (a keywordActionAdapter) UpdateBasicFilter(ctx context.Context, keywordID string, basicFilter *string) error {
	return a.service.UpdateBasicFilter(ctx, keywordID, basicFilter)
}

func (a keywordActionAdapter) SetTelegramEnabled(ctx context.Context, keywordID string, enabled bool) error {
	return a.service.SetTelegramEnabled(ctx, keywordID, enabled)
}
