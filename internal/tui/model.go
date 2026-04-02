package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pricealert/pricealert/internal/dto"
)

type QueryService interface {
	DashboardState(context.Context, *string) (*dto.DashboardState, error)
	KeywordDetail(context.Context, string) (*dto.KeywordDetail, error)
}

type RuntimeTrigger interface {
	RunOnce(context.Context) (RuntimeRunResult, error)
}

type KeywordActions interface {
	AddKeyword(context.Context, string) error
	PauseKeyword(context.Context, string) error
	ResumeKeyword(context.Context, string) error
	ArchiveKeyword(context.Context, string) error
	UpdateThreshold(context.Context, string, *int64) error
	UpdateInterval(context.Context, string, int) error
	UpdateBasicFilter(context.Context, string, *string) error
	SetTelegramEnabled(context.Context, string, bool) error
}

type RuntimeRunResult struct {
	Started []string
	Skipped []string
}

type screen string
type editField string

const (
	screenDashboard screen = "dashboard"
	screenDetail    screen = "detail"

	editFieldNone        editField = ""
	editFieldThreshold   editField = "threshold"
	editFieldInterval    editField = "interval"
	editFieldBasicFilter editField = "basic_filter"
)

type dashboardLoadedMsg struct {
	state *dto.DashboardState
	err   error
}

type detailLoadedMsg struct {
	detail *dto.KeywordDetail
	err    error
}

type refreshDoneMsg struct {
	screen  screen
	detail  *dto.KeywordDetail
	state   *dto.DashboardState
	status  string
	err     error
	runtime bool
}

type keywordActionDoneMsg struct {
	screen screen
	state  *dto.DashboardState
	detail *dto.KeywordDetail
	status string
	err    error
}

type Model struct {
	query             QueryService
	runtime           RuntimeTrigger
	keywords          KeywordActions
	screen            screen
	dashboard         *dto.DashboardState
	detail            *dto.KeywordDetail
	selectedIndex     int
	selectedKeywordID *string
	err               error
	loading           bool
	statusMessage     string
	addingKeyword     bool
	addKeywordInput   string
	editingField      editField
	editFieldInput    string
}

func NewModel(query QueryService, runtime RuntimeTrigger, keywords KeywordActions) Model {
	return Model{
		query:    query,
		runtime:  runtime,
		keywords: keywords,
		screen:   screenDashboard,
	}
}

func (m Model) Init() tea.Cmd {
	m.loading = true
	m.statusMessage = "Loading dashboard..."
	return loadDashboardCmd(m.query, nil)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case dashboardLoadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			m.statusMessage = "Dashboard load failed"
			return m, nil
		}
		m.dashboard = msg.state
		m.screen = screenDashboard
		m.selectedKeywordID = msg.state.SelectedKeywordID
		m.selectedIndex = selectedIndexForDashboard(msg.state)
		m.statusMessage = "Dashboard loaded"
		return m, nil
	case detailLoadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			m.statusMessage = "Detail load failed"
			return m, nil
		}
		m.detail = msg.detail
		m.screen = screenDetail
		m.statusMessage = "Detail loaded"
		return m, nil
	case keywordActionDoneMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			m.statusMessage = "Keyword action failed"
			return m, nil
		}
		m.addingKeyword = false
		m.addKeywordInput = ""
		m.editingField = editFieldNone
		m.editFieldInput = ""
		if msg.screen == screenDetail {
			m.detail = msg.detail
			m.screen = screenDetail
		} else {
			m.dashboard = msg.state
			m.screen = screenDashboard
			if msg.state != nil {
				m.selectedKeywordID = msg.state.SelectedKeywordID
				m.selectedIndex = selectedIndexForDashboard(msg.state)
			}
		}
		m.statusMessage = msg.status
		return m, nil
	case refreshDoneMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			if msg.runtime {
				m.statusMessage = "Refresh failed after runtime step"
			} else {
				m.statusMessage = "Refresh failed"
			}
			return m, nil
		}
		if msg.screen == screenDetail {
			m.detail = msg.detail
			m.screen = screenDetail
		} else {
			m.dashboard = msg.state
			m.screen = screenDashboard
			if msg.state != nil {
				m.selectedKeywordID = msg.state.SelectedKeywordID
				m.selectedIndex = selectedIndexForDashboard(msg.state)
			}
		}
		m.statusMessage = msg.status
		return m, nil
	}

	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.addingKeyword || m.editingField != editFieldNone {
		return m.updateAddKeyword(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}

	if m.screen == screenDetail {
		switch msg.String() {
		case "esc", "h", "backspace":
			m.loading = true
			m.statusMessage = "Loading dashboard..."
			return m, loadDashboardCmd(m.query, m.selectedKeywordID)
		case "r":
			m.loading = true
			m.statusMessage = "Refreshing detail..."
			if m.selectedKeywordID == nil {
				return m, nil
			}
			return m, refreshCurrentScreenCmd(m.query, m.runtime, screenDetail, m.selectedKeywordID)
		case "p":
			if m.detail != nil {
				m.loading = true
				m.statusMessage = "Pausing keyword..."
				return m, keywordActionCmd(m.query, m.keywords, keywordActionPause, screenDetail, m.detail.Keyword.ID, "", m.selectedKeywordID)
			}
		case "u":
			if m.detail != nil {
				m.loading = true
				m.statusMessage = "Resuming keyword..."
				return m, keywordActionCmd(m.query, m.keywords, keywordActionResume, screenDetail, m.detail.Keyword.ID, "", m.selectedKeywordID)
			}
		case "x":
			if m.detail != nil {
				m.loading = true
				m.statusMessage = "Archiving keyword..."
				return m, keywordActionCmd(m.query, m.keywords, keywordActionArchive, screenDashboard, m.detail.Keyword.ID, "", nil)
			}
		case "t":
			if m.detail != nil {
				m.loading = true
				m.statusMessage = "Updating telegram setting..."
				return m, toggleTelegramCmd(m.query, m.keywords, m.detail.Keyword.ID, !m.detail.Keyword.TelegramEnabled)
			}
		case "1":
			if m.detail != nil {
				m.editingField = editFieldThreshold
				m.editFieldInput = valueOrEmpty(m.detail.Keyword.ThresholdPrice)
				m.statusMessage = "Edit threshold and press enter"
				return m, nil
			}
		case "2":
			if m.detail != nil {
				m.editingField = editFieldInterval
				m.editFieldInput = fmt.Sprintf("%d", m.detail.Keyword.IntervalMinutes)
				m.statusMessage = "Edit interval and press enter"
				return m, nil
			}
		case "3":
			if m.detail != nil {
				m.editingField = editFieldBasicFilter
				m.editFieldInput = stringOrEmpty(m.detail.Keyword.BasicFilter)
				m.statusMessage = "Edit basic filter and press enter"
				return m, nil
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "j", "down":
		if keywordID, ok := m.nextSelection(1); ok {
			m.loading = true
			m.statusMessage = "Loading dashboard..."
			return m, loadDashboardCmd(m.query, &keywordID)
		}
	case "k", "up":
		if keywordID, ok := m.nextSelection(-1); ok {
			m.loading = true
			m.statusMessage = "Loading dashboard..."
			return m, loadDashboardCmd(m.query, &keywordID)
		}
	case "enter", "l":
		if m.selectedKeywordID != nil {
			m.loading = true
			m.statusMessage = "Loading detail..."
			return m, loadDetailCmd(m.query, *m.selectedKeywordID)
		}
	case "r":
		m.loading = true
		m.statusMessage = "Refreshing dashboard..."
		return m, refreshCurrentScreenCmd(m.query, m.runtime, screenDashboard, m.selectedKeywordID)
	case "a":
		m.addingKeyword = true
		m.addKeywordInput = ""
		m.statusMessage = "Add a keyword and press enter"
		return m, nil
	case "p":
		if m.selectedKeywordID != nil {
			m.loading = true
			m.statusMessage = "Pausing keyword..."
			return m, keywordActionCmd(m.query, m.keywords, keywordActionPause, screenDashboard, *m.selectedKeywordID, "", m.selectedKeywordID)
		}
	case "u":
		if m.selectedKeywordID != nil {
			m.loading = true
			m.statusMessage = "Resuming keyword..."
			return m, keywordActionCmd(m.query, m.keywords, keywordActionResume, screenDashboard, *m.selectedKeywordID, "", m.selectedKeywordID)
		}
	case "x":
		if m.selectedKeywordID != nil {
			m.loading = true
			m.statusMessage = "Archiving keyword..."
			return m, keywordActionCmd(m.query, m.keywords, keywordActionArchive, screenDashboard, *m.selectedKeywordID, "", nil)
		}
	}

	return m, nil
}

func (m Model) updateAddKeyword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.editingField != editFieldNone {
			m.editingField = editFieldNone
			m.editFieldInput = ""
			m.statusMessage = "Edit canceled"
			return m, nil
		}
		m.addingKeyword = false
		m.addKeywordInput = ""
		m.statusMessage = "Add keyword canceled"
		return m, nil
	case tea.KeyBackspace:
		if m.editingField != editFieldNone {
			if len(m.editFieldInput) > 0 {
				m.editFieldInput = m.editFieldInput[:len(m.editFieldInput)-1]
			}
			return m, nil
		}
		if len(m.addKeywordInput) > 0 {
			m.addKeywordInput = m.addKeywordInput[:len(m.addKeywordInput)-1]
		}
		return m, nil
	case tea.KeyEnter:
		if m.editingField != editFieldNone {
			return m.submitEditField()
		}
		m.loading = true
		m.statusMessage = "Adding keyword..."
		return m, keywordActionCmd(m.query, m.keywords, keywordActionAdd, screenDashboard, "", m.addKeywordInput, nil)
	}

	if len(msg.Runes) > 0 {
		if m.editingField != editFieldNone {
			m.editFieldInput += string(msg.Runes)
			return m, nil
		}
		m.addKeywordInput += string(msg.Runes)
	}

	return m, nil
}

func (m Model) submitEditField() (tea.Model, tea.Cmd) {
	if m.detail == nil {
		return m, nil
	}

	m.loading = true

	switch m.editingField {
	case editFieldThreshold:
		threshold, err := parseOptionalPositiveInt64(m.editFieldInput)
		if err != nil {
			m.loading = false
			m.err = err
			m.statusMessage = "Threshold update failed"
			return m, nil
		}
		m.statusMessage = "Updating threshold..."
		return m, updateThresholdCmd(m.query, m.keywords, m.detail.Keyword.ID, threshold)
	case editFieldInterval:
		interval, err := parseRequiredPositiveInt(m.editFieldInput)
		if err != nil {
			m.loading = false
			m.err = err
			m.statusMessage = "Interval update failed"
			return m, nil
		}
		m.statusMessage = "Updating interval..."
		return m, updateIntervalCmd(m.query, m.keywords, m.detail.Keyword.ID, interval)
	case editFieldBasicFilter:
		filter := optionalString(m.editFieldInput)
		m.statusMessage = "Updating basic filter..."
		return m, updateBasicFilterCmd(m.query, m.keywords, m.detail.Keyword.ID, filter)
	default:
		m.loading = false
		return m, nil
	}
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("PriceAlert\n\nStatus: %s\n\nError: %v\n\nPress r to retry or q to quit.", m.statusMessage, m.err)
	}

	switch m.screen {
	case screenDetail:
		return renderDetail(m.detail, m.loading, m.statusMessage, m.editingField, m.editFieldInput)
	default:
		return renderDashboard(m.dashboard, m.selectedIndex, m.loading, m.statusMessage, m.addingKeyword, m.addKeywordInput)
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

func refreshCurrentScreenCmd(query QueryService, runtime RuntimeTrigger, current screen, selectedKeywordID *string) tea.Cmd {
	return func() tea.Msg {
		result, err := runtime.RunOnce(context.Background())
		if err != nil {
			return refreshDoneMsg{screen: current, err: err, runtime: true}
		}

		status := fmt.Sprintf("Refresh complete. Runtime started %d, skipped %d.", len(result.Started), len(result.Skipped))
		if current == screenDetail && selectedKeywordID != nil {
			detail, err := query.KeywordDetail(context.Background(), *selectedKeywordID)
			return refreshDoneMsg{
				screen:  screenDetail,
				detail:  detail,
				status:  status,
				err:     err,
				runtime: true,
			}
		}

		state, err := query.DashboardState(context.Background(), selectedKeywordID)
		return refreshDoneMsg{
			screen:  screenDashboard,
			state:   state,
			status:  status,
			err:     err,
			runtime: true,
		}
	}
}

type keywordActionType string

const (
	keywordActionAdd     keywordActionType = "add"
	keywordActionPause   keywordActionType = "pause"
	keywordActionResume  keywordActionType = "resume"
	keywordActionArchive keywordActionType = "archive"
)

func updateThresholdCmd(query QueryService, keywords KeywordActions, keywordID string, threshold *int64) tea.Cmd {
	return func() tea.Msg {
		err := keywords.UpdateThreshold(context.Background(), keywordID, threshold)
		if err != nil {
			return keywordActionDoneMsg{screen: screenDetail, err: err}
		}

		detail, detailErr := query.KeywordDetail(context.Background(), keywordID)
		return keywordActionDoneMsg{
			screen: screenDetail,
			detail: detail,
			status: "Threshold updated",
			err:    detailErr,
		}
	}
}

func updateIntervalCmd(query QueryService, keywords KeywordActions, keywordID string, intervalMinutes int) tea.Cmd {
	return func() tea.Msg {
		err := keywords.UpdateInterval(context.Background(), keywordID, intervalMinutes)
		if err != nil {
			return keywordActionDoneMsg{screen: screenDetail, err: err}
		}

		detail, detailErr := query.KeywordDetail(context.Background(), keywordID)
		return keywordActionDoneMsg{
			screen: screenDetail,
			detail: detail,
			status: "Interval updated",
			err:    detailErr,
		}
	}
}

func updateBasicFilterCmd(query QueryService, keywords KeywordActions, keywordID string, basicFilter *string) tea.Cmd {
	return func() tea.Msg {
		err := keywords.UpdateBasicFilter(context.Background(), keywordID, basicFilter)
		if err != nil {
			return keywordActionDoneMsg{screen: screenDetail, err: err}
		}

		detail, detailErr := query.KeywordDetail(context.Background(), keywordID)
		return keywordActionDoneMsg{
			screen: screenDetail,
			detail: detail,
			status: "Basic filter updated",
			err:    detailErr,
		}
	}
}

func toggleTelegramCmd(query QueryService, keywords KeywordActions, keywordID string, enabled bool) tea.Cmd {
	return func() tea.Msg {
		err := keywords.SetTelegramEnabled(context.Background(), keywordID, enabled)
		if err != nil {
			return keywordActionDoneMsg{screen: screenDetail, err: err}
		}

		detail, detailErr := query.KeywordDetail(context.Background(), keywordID)
		return keywordActionDoneMsg{
			screen: screenDetail,
			detail: detail,
			status: "Telegram setting updated",
			err:    detailErr,
		}
	}
}

func keywordActionCmd(
	query QueryService,
	keywords KeywordActions,
	action keywordActionType,
	targetScreen screen,
	keywordID string,
	keywordText string,
	selectedKeywordID *string,
) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch action {
		case keywordActionAdd:
			err = keywords.AddKeyword(context.Background(), keywordText)
		case keywordActionPause:
			err = keywords.PauseKeyword(context.Background(), keywordID)
		case keywordActionResume:
			err = keywords.ResumeKeyword(context.Background(), keywordID)
		case keywordActionArchive:
			err = keywords.ArchiveKeyword(context.Background(), keywordID)
		}
		if err != nil {
			return keywordActionDoneMsg{screen: targetScreen, err: err}
		}

		if targetScreen == screenDetail && keywordID != "" {
			detail, detailErr := query.KeywordDetail(context.Background(), keywordID)
			return keywordActionDoneMsg{
				screen: screenDetail,
				detail: detail,
				status: keywordActionStatus(action),
				err:    detailErr,
			}
		}

		state, stateErr := query.DashboardState(context.Background(), selectedKeywordID)
		return keywordActionDoneMsg{
			screen: screenDashboard,
			state:  state,
			status: keywordActionStatus(action),
			err:    stateErr,
		}
	}
}

func keywordActionStatus(action keywordActionType) string {
	switch action {
	case keywordActionAdd:
		return "Keyword added"
	case keywordActionPause:
		return "Keyword paused"
	case keywordActionResume:
		return "Keyword resumed"
	case keywordActionArchive:
		return "Keyword archived"
	default:
		return "Keyword updated"
	}
}

func renderDashboard(state *dto.DashboardState, selectedIndex int, loading bool, status string, adding bool, input string) string {
	if state == nil {
		return fmt.Sprintf("PriceAlert\n\nStatus: %s", status)
	}

	var lines []string
	lines = append(lines, "PriceAlert", "", "Dashboard", "")
	lines = append(lines, fmt.Sprintf("Status: %s", status))
	if loading {
		lines = append(lines, "Loading: yes")
	}
	if adding {
		lines = append(lines, fmt.Sprintf("Add Keyword: %s", input))
		lines = append(lines, "Press enter to save, esc to cancel")
	}
	lines = append(lines, "", "Runtime:")
	lines = append(lines, formatRuntimeStatus(state.RuntimeStatus)...)
	lines = append(lines, "")
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

	lines = append(lines, "", "Keys: j/k move, enter detail, a add, p pause, u resume, x archive, r refresh, q quit")
	return strings.Join(lines, "\n")
}

func formatRuntimeStatus(status *dto.RuntimeStatusSummary) []string {
	if status == nil {
		return []string{"  (runtime status unavailable)"}
	}

	accepting := "no"
	if status.AcceptingNewWork {
		accepting = "yes"
	}

	lines := []string{
		fmt.Sprintf("  Accepting New Work: %s", accepting),
		fmt.Sprintf("  Running: %d / %d", status.RunningCount, status.MaxConcurrent),
	}

	if status.ReconciledRunningJobs > 0 {
		lines = append(lines, fmt.Sprintf("  Startup Reconciled: %d running job(s)", status.ReconciledRunningJobs))
	}
	if status.PrunedRawListings > 0 {
		lines = append(lines, fmt.Sprintf("  Startup Pruned Raw Listings: %d", status.PrunedRawListings))
	}
	if status.PrunedAlertEvents > 0 {
		lines = append(lines, fmt.Sprintf("  Startup Pruned Alert Events: %d", status.PrunedAlertEvents))
	}
	if status.PrunedPricePoints > 0 {
		lines = append(lines, fmt.Sprintf("  Startup Pruned Price Points: %d", status.PrunedPricePoints))
	}

	return lines
}

func renderDetail(detail *dto.KeywordDetail, loading bool, status string, editingField editField, editInput string) string {
	if detail == nil {
		return fmt.Sprintf("PriceAlert\n\nStatus: %s", status)
	}

	var lines []string
	lines = append(lines, "PriceAlert", "", "Keyword Detail", "")
	lines = append(lines, fmt.Sprintf("Status: %s", status))
	if loading {
		lines = append(lines, "Loading: yes")
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Keyword: %s", detail.Keyword.Keyword))
	lines = append(lines, fmt.Sprintf("Status: %s", detail.Keyword.Status))
	lines = append(lines, fmt.Sprintf("Interval: %d min", detail.Keyword.IntervalMinutes))
	lines = append(lines, fmt.Sprintf("Threshold: %s", formatPrice(detail.Keyword.ThresholdPrice)))
	lines = append(lines, fmt.Sprintf("Basic Filter: %s", formatOptionalString(detail.Keyword.BasicFilter)))
	lines = append(lines, fmt.Sprintf("Telegram Enabled: %t", detail.Keyword.TelegramEnabled))
	if editingField != editFieldNone {
		lines = append(lines, fmt.Sprintf("Editing %s: %s", editingFieldLabel(editingField), editInput))
		lines = append(lines, "Press enter to save, esc to cancel")
	}

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

	lines = append(lines, "", "Keys: esc back, p pause, u resume, x archive, t telegram, 1 threshold, 2 interval, 3 filter, r refresh, q quit")
	return strings.Join(lines, "\n")
}

func formatPrice(value *int64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}

func formatOptionalString(value *string) string {
	if value == nil {
		return "-"
	}
	return *value
}

func editingFieldLabel(field editField) string {
	switch field {
	case editFieldThreshold:
		return "threshold"
	case editFieldInterval:
		return "interval"
	case editFieldBasicFilter:
		return "basic filter"
	default:
		return "field"
	}
}

func valueOrEmpty(value *int64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func parseOptionalPositiveInt64(value string) (*int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("threshold price must be empty or a positive integer")
	}
	if parsed <= 0 {
		return nil, fmt.Errorf("threshold price must be empty or a positive integer")
	}
	return &parsed, nil
}

func parseRequiredPositiveInt(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("interval minutes must be a positive integer")
	}
	return parsed, nil
}
