package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pricealert/pricealert/internal/dto"
)

type QueryService interface {
	DashboardState(context.Context, *string) (*dto.DashboardState, error)
	KeywordDetail(context.Context, string) (*dto.KeywordDetail, error)
}

type screen string

const (
	screenDashboard screen = "dashboard"
	screenDetail    screen = "detail"
)

type dashboardLoadedMsg struct {
	state *dto.DashboardState
	err   error
}

type detailLoadedMsg struct {
	detail *dto.KeywordDetail
	err    error
}

type Model struct {
	query             QueryService
	screen            screen
	dashboard         *dto.DashboardState
	detail            *dto.KeywordDetail
	selectedIndex     int
	selectedKeywordID *string
	err               error
}

func NewModel(query QueryService) Model {
	return Model{
		query:  query,
		screen: screenDashboard,
	}
}

func (m Model) Init() tea.Cmd {
	return loadDashboardCmd(m.query, nil)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case dashboardLoadedMsg:
		m.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		m.dashboard = msg.state
		m.screen = screenDashboard
		m.selectedKeywordID = msg.state.SelectedKeywordID
		m.selectedIndex = selectedIndexForDashboard(msg.state)
		return m, nil
	case detailLoadedMsg:
		m.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		m.detail = msg.detail
		m.screen = screenDetail
		return m, nil
	}

	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}

	if m.screen == screenDetail {
		switch msg.String() {
		case "esc", "h", "backspace":
			return m, loadDashboardCmd(m.query, m.selectedKeywordID)
		}
		return m, nil
	}

	switch msg.String() {
	case "j", "down":
		if keywordID, ok := m.nextSelection(1); ok {
			return m, loadDashboardCmd(m.query, &keywordID)
		}
	case "k", "up":
		if keywordID, ok := m.nextSelection(-1); ok {
			return m, loadDashboardCmd(m.query, &keywordID)
		}
	case "enter", "l":
		if m.selectedKeywordID != nil {
			return m, loadDetailCmd(m.query, *m.selectedKeywordID)
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("PriceAlert\n\nError: %v\n\nPress q to quit.", m.err)
	}

	switch m.screen {
	case screenDetail:
		return renderDetail(m.detail)
	default:
		return renderDashboard(m.dashboard, m.selectedIndex)
	}
}

func (m Model) nextSelection(offset int) (string, bool) {
	if m.dashboard == nil || len(m.dashboard.TrackedKeywords) == 0 {
		return "", false
	}

	next := m.selectedIndex + offset
	if next < 0 {
		next = 0
	}
	if next >= len(m.dashboard.TrackedKeywords) {
		next = len(m.dashboard.TrackedKeywords) - 1
	}

	if next == m.selectedIndex && m.selectedKeywordID != nil {
		return *m.selectedKeywordID, true
	}

	return m.dashboard.TrackedKeywords[next].ID, true
}

func selectedIndexForDashboard(state *dto.DashboardState) int {
	if state == nil || len(state.TrackedKeywords) == 0 {
		return 0
	}
	if state.SelectedKeywordID == nil {
		return 0
	}
	for index, keyword := range state.TrackedKeywords {
		if keyword.ID == *state.SelectedKeywordID {
			return index
		}
	}
	return 0
}

func loadDashboardCmd(query QueryService, selectedKeywordID *string) tea.Cmd {
	return func() tea.Msg {
		state, err := query.DashboardState(context.Background(), selectedKeywordID)
		return dashboardLoadedMsg{state: state, err: err}
	}
}

func loadDetailCmd(query QueryService, keywordID string) tea.Cmd {
	return func() tea.Msg {
		detail, err := query.KeywordDetail(context.Background(), keywordID)
		return detailLoadedMsg{detail: detail, err: err}
	}
}

func renderDashboard(state *dto.DashboardState, selectedIndex int) string {
	if state == nil {
		return "PriceAlert\n\nLoading dashboard..."
	}

	var lines []string
	lines = append(lines, "PriceAlert", "", "Dashboard", "")
	lines = append(lines, "Keywords:")
	if len(state.TrackedKeywords) == 0 {
		lines = append(lines, "  (no tracked keywords)")
	} else {
		for index, keyword := range state.TrackedKeywords {
			prefix := "  "
			if index == selectedIndex {
				prefix = "> "
			}
			alertMarker := ""
			if keyword.HasNewAlert {
				alertMarker = " [alert]"
			}
			lines = append(lines, fmt.Sprintf("%s%s (%s)%s", prefix, keyword.Keyword, keyword.Status, alertMarker))
		}
	}

	lines = append(lines, "", "Selected Snapshot:")
	if state.SelectedSnapshot == nil {
		lines = append(lines, "  (no snapshot yet)")
	} else {
		lines = append(lines, fmt.Sprintf("  Signal: %s", state.SelectedSnapshot.Signal))
		lines = append(lines, fmt.Sprintf("  Min/Avg/Max: %s / %s / %s",
			formatPrice(state.SelectedSnapshot.MinPrice),
			formatPrice(state.SelectedSnapshot.AvgPrice),
			formatPrice(state.SelectedSnapshot.MaxPrice),
		))
		lines = append(lines, fmt.Sprintf("  Raw/Grouped: %d / %d", state.SelectedSnapshot.RawCount, state.SelectedSnapshot.GroupedCount))
	}

	lines = append(lines, "", fmt.Sprintf("Recent Events: %d", len(state.RecentEvents)))
	for _, event := range state.RecentEvents {
		lines = append(lines, fmt.Sprintf("  - [%s] %s", event.EventType, event.Message))
	}

	lines = append(lines, "", "Keys: j/k move, enter detail, q quit")
	return strings.Join(lines, "\n")
}

func renderDetail(detail *dto.KeywordDetail) string {
	if detail == nil {
		return "PriceAlert\n\nLoading detail..."
	}

	var lines []string
	lines = append(lines, "PriceAlert", "", "Keyword Detail", "")
	lines = append(lines, fmt.Sprintf("Keyword: %s", detail.Keyword.Keyword))
	lines = append(lines, fmt.Sprintf("Status: %s", detail.Keyword.Status))
	lines = append(lines, fmt.Sprintf("Interval: %d min", detail.Keyword.IntervalMinutes))
	lines = append(lines, fmt.Sprintf("Threshold: %s", formatPrice(detail.Keyword.ThresholdPrice)))

	lines = append(lines, "", "Snapshot:")
	if detail.Snapshot == nil {
		lines = append(lines, "  (no snapshot yet)")
	} else {
		lines = append(lines, fmt.Sprintf("  Signal: %s", detail.Snapshot.Signal))
		lines = append(lines, fmt.Sprintf("  Min/Avg/Max: %s / %s / %s",
			formatPrice(detail.Snapshot.MinPrice),
			formatPrice(detail.Snapshot.AvgPrice),
			formatPrice(detail.Snapshot.MaxPrice),
		))
	}

	lines = append(lines, "", "Top Deals:")
	if len(detail.TopDeals) == 0 {
		lines = append(lines, "  (no grouped listings)")
	} else {
		for _, deal := range detail.TopDeals {
			lines = append(lines, fmt.Sprintf("  - %s | %s | %d listings", deal.RepresentativeTitle, formatPrice(&deal.BestPrice), deal.ListingCount))
		}
	}

	lines = append(lines, "", "Recent History:")
	if len(detail.RecentHistory) == 0 {
		lines = append(lines, "  (no price history)")
	} else {
		for _, point := range detail.RecentHistory {
			lines = append(lines, fmt.Sprintf("  - %s / %s / %s",
				formatPrice(point.MinPrice),
				formatPrice(point.AvgPrice),
				formatPrice(point.MaxPrice),
			))
		}
	}

	lines = append(lines, "", "Recent Events:")
	if len(detail.RecentEvents) == 0 {
		lines = append(lines, "  (no recent events)")
	} else {
		for _, event := range detail.RecentEvents {
			lines = append(lines, fmt.Sprintf("  - [%s] %s", event.EventType, event.Message))
		}
	}

	lines = append(lines, "", "Keys: esc back, q quit")
	return strings.Join(lines, "\n")
}

func formatPrice(value *int64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}
