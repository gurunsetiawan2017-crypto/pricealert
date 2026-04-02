package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pricealert/pricealert/internal/dto"
)

func TestModelInitLoadsDashboard(t *testing.T) {
	query := &fakeQueryService{
		dashboard: &dto.DashboardState{
			TrackedKeywords: []dto.TrackedKeywordSummary{
				{ID: "kw_1", Keyword: "minyak goreng 2L", Status: "active"},
			},
			SelectedKeywordID: stringPtr("kw_1"),
		},
	}

	model := NewModel(query, fakeRuntimeTrigger{}, &fakeKeywordActions{})
	msg := model.Init()()
	updated, _ := model.Update(msg)
	got := updated.(Model)

	if got.dashboard == nil {
		t.Fatalf("dashboard not loaded")
	}
	if got.selectedKeywordID == nil || *got.selectedKeywordID != "kw_1" {
		t.Fatalf("selected keyword id = %v", got.selectedKeywordID)
	}
}

func TestDashboardSelectionMovesAndLoadsDetail(t *testing.T) {
	query := &fakeQueryService{
		dashboard: &dto.DashboardState{
			TrackedKeywords: []dto.TrackedKeywordSummary{
				{ID: "kw_1", Keyword: "minyak goreng 2L", Status: "active"},
				{ID: "kw_2", Keyword: "gula pasir 1kg", Status: "paused"},
			},
			SelectedKeywordID: stringPtr("kw_1"),
		},
		detailByKeywordID: map[string]*dto.KeywordDetail{
			"kw_2": {
				Keyword: dto.TrackedKeyword{ID: "kw_2", Keyword: "gula pasir 1kg", Status: "paused"},
			},
		},
	}

	model := NewModel(query, fakeRuntimeTrigger{}, &fakeKeywordActions{})
	updated, cmd := model.Update(dashboardLoadedMsg{state: query.dashboard})
	m := updated.(Model)

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd == nil {
		t.Fatalf("expected dashboard reload cmd")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.selectedKeywordID == nil || *m.selectedKeywordID != "kw_2" {
		t.Fatalf("selected keyword id = %v, want kw_2", m.selectedKeywordID)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected detail load cmd")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.screen != screenDetail {
		t.Fatalf("screen = %s, want %s", m.screen, screenDetail)
	}
	if m.detail == nil || m.detail.Keyword.ID != "kw_2" {
		t.Fatalf("detail = %#v", m.detail)
	}
}

func TestDetailEscReturnsToDashboard(t *testing.T) {
	query := &fakeQueryService{
		dashboard: &dto.DashboardState{
			TrackedKeywords:   []dto.TrackedKeywordSummary{{ID: "kw_1", Keyword: "minyak goreng 2L", Status: "active"}},
			SelectedKeywordID: stringPtr("kw_1"),
		},
	}

	model := NewModel(query, fakeRuntimeTrigger{}, &fakeKeywordActions{})
	model.screen = screenDetail
	model.selectedKeywordID = stringPtr("kw_1")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected dashboard reload cmd")
	}
	updated, _ = updated.(Model).Update(cmd())
	m := updated.(Model)
	if m.screen != screenDashboard {
		t.Fatalf("screen = %s, want %s", m.screen, screenDashboard)
	}
}

func TestViewShowsError(t *testing.T) {
	model := NewModel(&fakeQueryService{}, fakeRuntimeTrigger{}, &fakeKeywordActions{})
	model.err = errors.New("load failed")

	if !strings.Contains(model.View(), "load failed") {
		t.Fatalf("view does not contain error: %q", model.View())
	}
}

func TestRefreshRunsRuntimeAndReloadsDashboard(t *testing.T) {
	query := &fakeQueryService{
		dashboard: &dto.DashboardState{
			TrackedKeywords:   []dto.TrackedKeywordSummary{{ID: "kw_1", Keyword: "minyak goreng 2L", Status: "active"}},
			SelectedKeywordID: stringPtr("kw_1"),
		},
	}
	runtime := fakeRuntimeTrigger{result: RuntimeRunResult{Started: []string{"kw_1"}, Skipped: []string{}}}

	model := NewModel(query, runtime, &fakeKeywordActions{})
	updated, cmd := model.Update(dashboardLoadedMsg{state: query.dashboard})
	m := updated.(Model)

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatalf("expected refresh cmd")
	}
	msg := cmd()
	updated, _ = updated.(Model).Update(msg)
	m = updated.(Model)

	if runtime.calls != 0 {
		t.Fatalf("runtime calls tracked on value copy unexpectedly")
	}
	if m.loading {
		t.Fatalf("expected loading false after refresh")
	}
	if !strings.Contains(m.statusMessage, "Runtime started 1") {
		t.Fatalf("status message = %q", m.statusMessage)
	}
}

func TestRefreshErrorShowsStatus(t *testing.T) {
	model := NewModel(&fakeQueryService{}, fakeRuntimeTrigger{err: errors.New("runtime failed")}, &fakeKeywordActions{})
	model.screen = screenDashboard

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatalf("expected refresh cmd")
	}
	updated, _ = updated.(Model).Update(cmd())
	m := updated.(Model)

	if m.err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(m.statusMessage, "Refresh failed") {
		t.Fatalf("status message = %q", m.statusMessage)
	}
}

func TestAddKeywordInputAndSubmit(t *testing.T) {
	query := &fakeQueryService{
		dashboard: &dto.DashboardState{
			TrackedKeywords:   []dto.TrackedKeywordSummary{{ID: "kw_1", Keyword: "existing", Status: "active"}},
			SelectedKeywordID: stringPtr("kw_1"),
		},
	}
	actions := &fakeKeywordActions{}
	model := NewModel(query, fakeRuntimeTrigger{}, actions)

	updated, _ := model.Update(dashboardLoadedMsg{state: query.dashboard})
	m := updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(Model)
	if !m.addingKeyword {
		t.Fatalf("expected add mode")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("baru")})
	m = updated.(Model)
	if m.addKeywordInput != "baru" {
		t.Fatalf("input = %q", m.addKeywordInput)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected add command")
	}
	updated, _ = updated.(Model).Update(cmd())
	m = updated.(Model)

	if actions.addedKeyword != "baru" {
		t.Fatalf("added keyword = %q", actions.addedKeyword)
	}
	if m.addingKeyword {
		t.Fatalf("expected add mode to end")
	}
	if !strings.Contains(m.statusMessage, "added") {
		t.Fatalf("status message = %q", m.statusMessage)
	}
}

func TestPauseSelectedKeywordFromDashboard(t *testing.T) {
	query := &fakeQueryService{
		dashboard: &dto.DashboardState{
			TrackedKeywords:   []dto.TrackedKeywordSummary{{ID: "kw_1", Keyword: "existing", Status: "active"}},
			SelectedKeywordID: stringPtr("kw_1"),
		},
	}
	actions := &fakeKeywordActions{}
	model := NewModel(query, fakeRuntimeTrigger{}, actions)

	updated, _ := model.Update(dashboardLoadedMsg{state: query.dashboard})
	m := updated.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatalf("expected pause command")
	}
	updated, _ = updated.(Model).Update(cmd())
	m = updated.(Model)

	if actions.pausedKeywordID != "kw_1" {
		t.Fatalf("paused keyword id = %q", actions.pausedKeywordID)
	}
	if !strings.Contains(m.statusMessage, "paused") {
		t.Fatalf("status message = %q", m.statusMessage)
	}
}

func TestDashboardViewShowsRuntimeStatusSummary(t *testing.T) {
	model := NewModel(&fakeQueryService{}, fakeRuntimeTrigger{}, &fakeKeywordActions{})
	model.dashboard = &dto.DashboardState{
		RuntimeStatus: &dto.RuntimeStatusSummary{
			AcceptingNewWork:      true,
			RunningCount:          1,
			MaxConcurrent:         2,
			ReconciledRunningJobs: 3,
			PrunedRawListings:     9,
			PrunedAlertEvents:     5,
			PrunedPricePoints:     4,
		},
	}

	view := model.View()
	if !strings.Contains(view, "Runtime:") {
		t.Fatalf("view = %q", view)
	}
	if !strings.Contains(view, "Accepting New Work: yes") {
		t.Fatalf("view = %q", view)
	}
	if !strings.Contains(view, "Running: 1 / 2") {
		t.Fatalf("view = %q", view)
	}
	if !strings.Contains(view, "Startup Reconciled: 3 running job(s)") {
		t.Fatalf("view = %q", view)
	}
	if !strings.Contains(view, "Startup Pruned Raw Listings: 9") {
		t.Fatalf("view = %q", view)
	}
	if !strings.Contains(view, "Startup Pruned Alert Events: 5") {
		t.Fatalf("view = %q", view)
	}
	if !strings.Contains(view, "Startup Pruned Price Points: 4") {
		t.Fatalf("view = %q", view)
	}
}

func TestEditThresholdFromDetail(t *testing.T) {
	query := &fakeQueryService{
		detailByKeywordID: map[string]*dto.KeywordDetail{
			"kw_1": {
				Keyword: dto.TrackedKeyword{
					ID:              "kw_1",
					Keyword:         "minyak goreng 2L",
					Status:          "active",
					IntervalMinutes: 15,
				},
			},
		},
	}
	actions := &fakeKeywordActions{}
	model := NewModel(query, fakeRuntimeTrigger{}, actions)
	model.screen = screenDetail
	model.detail = query.detailByKeywordID["kw_1"]
	model.selectedKeywordID = stringPtr("kw_1")

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := updated.(Model)
	if m.editingField != editFieldThreshold {
		t.Fatalf("editing field = %q", m.editingField)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("25000")})
	m = updated.(Model)
	if m.editFieldInput != "25000" {
		t.Fatalf("edit input = %q", m.editFieldInput)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected threshold update command")
	}
	updated, _ = updated.(Model).Update(cmd())
	m = updated.(Model)

	if actions.thresholdKeywordID != "kw_1" {
		t.Fatalf("threshold keyword id = %q", actions.thresholdKeywordID)
	}
	if actions.threshold == nil || *actions.threshold != 25000 {
		t.Fatalf("threshold = %v", actions.threshold)
	}
	if m.editingField != editFieldNone {
		t.Fatalf("expected edit mode to end")
	}
	if !strings.Contains(m.statusMessage, "Threshold updated") {
		t.Fatalf("status message = %q", m.statusMessage)
	}
}

func TestEditIntervalValidationShowsError(t *testing.T) {
	query := &fakeQueryService{
		detailByKeywordID: map[string]*dto.KeywordDetail{
			"kw_1": {
				Keyword: dto.TrackedKeyword{
					ID:              "kw_1",
					Keyword:         "minyak goreng 2L",
					Status:          "active",
					IntervalMinutes: 15,
				},
			},
		},
	}
	model := NewModel(query, fakeRuntimeTrigger{}, &fakeKeywordActions{})
	model.screen = screenDetail
	model.detail = query.detailByKeywordID["kw_1"]

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m := updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	m = updated.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected no command on validation error")
	}
	m = updated.(Model)

	if m.err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(m.statusMessage, "Interval update failed") {
		t.Fatalf("status message = %q", m.statusMessage)
	}
}

func TestToggleTelegramFromDetail(t *testing.T) {
	query := &fakeQueryService{
		detailByKeywordID: map[string]*dto.KeywordDetail{
			"kw_1": {
				Keyword: dto.TrackedKeyword{
					ID:              "kw_1",
					Keyword:         "minyak goreng 2L",
					Status:          "active",
					IntervalMinutes: 15,
				},
			},
		},
	}
	actions := &fakeKeywordActions{}
	model := NewModel(query, fakeRuntimeTrigger{}, actions)
	model.screen = screenDetail
	model.detail = query.detailByKeywordID["kw_1"]

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if cmd == nil {
		t.Fatalf("expected toggle command")
	}
	updated, _ = updated.(Model).Update(cmd())
	m := updated.(Model)

	if actions.telegramKeywordID != "kw_1" {
		t.Fatalf("telegram keyword id = %q", actions.telegramKeywordID)
	}
	if !actions.telegramEnabled {
		t.Fatalf("telegram enabled not set")
	}
	if !strings.Contains(m.statusMessage, "Telegram setting updated") {
		t.Fatalf("status message = %q", m.statusMessage)
	}
}

type fakeQueryService struct {
	dashboard         *dto.DashboardState
	dashboardErr      error
	detailByKeywordID map[string]*dto.KeywordDetail
	detailErr         error
}

type fakeRuntimeTrigger struct {
	result RuntimeRunResult
	err    error
	calls  int
}

type fakeKeywordActions struct {
	addedKeyword         string
	pausedKeywordID      string
	resumedKeywordID     string
	archivedKeywordID    string
	thresholdKeywordID   string
	threshold            *int64
	intervalKeywordID    string
	interval             int
	basicFilterKeywordID string
	basicFilter          *string
	telegramKeywordID    string
	telegramEnabled      bool
	err                  error
}

func (f *fakeQueryService) DashboardState(_ context.Context, selectedKeywordID *string) (*dto.DashboardState, error) {
	if f.dashboardErr != nil {
		return nil, f.dashboardErr
	}
	state := cloneDashboard(f.dashboard)
	if state != nil && selectedKeywordID != nil {
		selectedID := *selectedKeywordID
		state.SelectedKeywordID = &selectedID
	}
	return state, nil
}

func (f *fakeQueryService) KeywordDetail(_ context.Context, keywordID string) (*dto.KeywordDetail, error) {
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	return f.detailByKeywordID[keywordID], nil
}

func (f fakeRuntimeTrigger) RunOnce(context.Context) (RuntimeRunResult, error) {
	return f.result, f.err
}

func (f *fakeKeywordActions) AddKeyword(_ context.Context, keyword string) error {
	f.addedKeyword = keyword
	return f.err
}

func (f *fakeKeywordActions) PauseKeyword(_ context.Context, keywordID string) error {
	f.pausedKeywordID = keywordID
	return f.err
}

func (f *fakeKeywordActions) ResumeKeyword(_ context.Context, keywordID string) error {
	f.resumedKeywordID = keywordID
	return f.err
}

func (f *fakeKeywordActions) ArchiveKeyword(_ context.Context, keywordID string) error {
	f.archivedKeywordID = keywordID
	return f.err
}

func (f *fakeKeywordActions) UpdateThreshold(_ context.Context, keywordID string, threshold *int64) error {
	f.thresholdKeywordID = keywordID
	f.threshold = threshold
	return f.err
}

func (f *fakeKeywordActions) UpdateInterval(_ context.Context, keywordID string, interval int) error {
	f.intervalKeywordID = keywordID
	f.interval = interval
	return f.err
}

func (f *fakeKeywordActions) UpdateBasicFilter(_ context.Context, keywordID string, basicFilter *string) error {
	f.basicFilterKeywordID = keywordID
	f.basicFilter = basicFilter
	return f.err
}

func (f *fakeKeywordActions) SetTelegramEnabled(_ context.Context, keywordID string, enabled bool) error {
	f.telegramKeywordID = keywordID
	f.telegramEnabled = enabled
	return f.err
}

func stringPtr(value string) *string {
	return &value
}

func cloneDashboard(state *dto.DashboardState) *dto.DashboardState {
	if state == nil {
		return nil
	}

	cloned := *state
	if state.SelectedKeywordID != nil {
		selectedID := *state.SelectedKeywordID
		cloned.SelectedKeywordID = &selectedID
	}
	cloned.TrackedKeywords = append([]dto.TrackedKeywordSummary(nil), state.TrackedKeywords...)
	cloned.TopDeals = append([]dto.GroupedListing(nil), state.TopDeals...)
	cloned.RecentEvents = append([]dto.AlertEvent(nil), state.RecentEvents...)
	if state.RuntimeStatus != nil {
		runtimeStatus := *state.RuntimeStatus
		cloned.RuntimeStatus = &runtimeStatus
	}
	return &cloned
}
