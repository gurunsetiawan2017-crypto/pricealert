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

	model := NewModel(query)
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

	model := NewModel(query)
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

	model := NewModel(query)
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
	model := NewModel(&fakeQueryService{})
	model.err = errors.New("load failed")

	if !strings.Contains(model.View(), "load failed") {
		t.Fatalf("view does not contain error: %q", model.View())
	}
}

type fakeQueryService struct {
	dashboard         *dto.DashboardState
	dashboardErr      error
	detailByKeywordID map[string]*dto.KeywordDetail
	detailErr         error
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
	return &cloned
}
