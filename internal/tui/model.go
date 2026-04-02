package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pricealert/pricealert/internal/dto"
)

type QueryService interface {
	DashboardState(context.Context, *string) (*dto.DashboardState, error)
	KeywordDetail(context.Context, string) (*dto.KeywordDetail, error)
}

type RuntimeTrigger interface {
	RunOnce(context.Context) (RuntimeRunResult, error)
	ScanNow(context.Context, string) error
}

type BrowserOpener interface {
	OpenURL(context.Context, string) error
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

type forceScanDoneMsg struct {
	screen screen
	detail *dto.KeywordDetail
	state  *dto.DashboardState
	status string
	err    error
}

type keywordActionDoneMsg struct {
	screen screen
	state  *dto.DashboardState
	detail *dto.KeywordDetail
	status string
	err    error
}

type openURLDoneMsg struct {
	url string
	err error
}

type dashboardFocus string
type detailFocus string

const (
	dashboardFocusKeywords dashboardFocus = "keywords"
	dashboardFocusDeals    dashboardFocus = "deals"

	detailFocusDeals   detailFocus = "deals"
	detailFocusHistory detailFocus = "history"
)

type Model struct {
	query              QueryService
	runtime            RuntimeTrigger
	browser            BrowserOpener
	keywords           KeywordActions
	screen             screen
	dashboard          *dto.DashboardState
	detail             *dto.KeywordDetail
	selectedIndex      int
	selectedKeywordID  *string
	err                error
	loading            bool
	statusMessage      string
	addingKeyword      bool
	addKeywordInput    string
	editingField       editField
	editFieldInput     string
	width              int
	height             int
	dashboardFocus     dashboardFocus
	detailFocus        detailFocus
	dashboardDealIndex int
	detailDealIndex    int
}

func NewModel(query QueryService, runtime RuntimeTrigger, browser BrowserOpener, keywords KeywordActions) Model {
	return Model{
		query:          query,
		runtime:        runtime,
		browser:        browser,
		keywords:       keywords,
		screen:         screenDashboard,
		width:          120,
		height:         36,
		dashboardFocus: dashboardFocusKeywords,
		detailFocus:    detailFocusDeals,
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
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
		m.dashboardDealIndex = clampIndex(m.dashboardDealIndex, len(msg.state.TopDeals))
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
		if msg.detail != nil {
			m.detailDealIndex = clampIndex(m.detailDealIndex, len(msg.detail.TopDeals))
		}
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
	case forceScanDoneMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			m.statusMessage = "Force scan failed"
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
	case openURLDoneMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			m.statusMessage = "Open link failed"
			return m, nil
		}
		m.statusMessage = "Opened link in browser"
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
		case "tab":
			m.detailFocus = nextDetailFocus(m.detailFocus)
			m.statusMessage = fmt.Sprintf("Focus: %s", m.detailFocus)
			return m, nil
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
		case "s":
			if m.detail != nil {
				m.loading = true
				m.statusMessage = "Running scan now..."
				return m, forceScanCmd(m.query, m.runtime, screenDetail, m.detail.Keyword.ID)
			}
		case "j", "down":
			if m.detailFocus == detailFocusDeals {
				m.detailDealIndex = clampIndex(m.detailDealIndex+1, len(m.currentDetailDeals()))
			}
			return m, nil
		case "k", "up":
			if m.detailFocus == detailFocusDeals {
				m.detailDealIndex = clampIndex(m.detailDealIndex-1, len(m.currentDetailDeals()))
			}
			return m, nil
		case "enter", "l":
			if m.detailFocus == detailFocusDeals {
				if url, ok := m.currentDetailDealURL(); ok {
					m.loading = true
					m.statusMessage = "Opening deal in browser..."
					return m, openURLCmd(m.browser, url)
				}
			}
			return m, nil
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
	case "tab":
		m.dashboardFocus = nextDashboardFocus(m.dashboardFocus)
		m.statusMessage = fmt.Sprintf("Focus: %s", m.dashboardFocus)
		return m, nil
	case "j", "down":
		if m.dashboardFocus == dashboardFocusDeals {
			m.dashboardDealIndex = clampIndex(m.dashboardDealIndex+1, len(m.currentDashboardDeals()))
			return m, nil
		}
		if keywordID, ok := m.nextSelection(1); ok {
			m.loading = true
			m.statusMessage = "Loading dashboard..."
			return m, loadDashboardCmd(m.query, &keywordID)
		}
	case "k", "up":
		if m.dashboardFocus == dashboardFocusDeals {
			m.dashboardDealIndex = clampIndex(m.dashboardDealIndex-1, len(m.currentDashboardDeals()))
			return m, nil
		}
		if keywordID, ok := m.nextSelection(-1); ok {
			m.loading = true
			m.statusMessage = "Loading dashboard..."
			return m, loadDashboardCmd(m.query, &keywordID)
		}
	case "enter", "l":
		if m.dashboardFocus == dashboardFocusDeals {
			if url, ok := m.currentDashboardDealURL(); ok {
				m.loading = true
				m.statusMessage = "Opening deal in browser..."
				return m, openURLCmd(m.browser, url)
			}
			return m, nil
		}
		if m.selectedKeywordID != nil {
			m.loading = true
			m.statusMessage = "Loading detail..."
			return m, loadDetailCmd(m.query, *m.selectedKeywordID)
		}
	case "r":
		m.loading = true
		m.statusMessage = "Refreshing dashboard..."
		return m, refreshCurrentScreenCmd(m.query, m.runtime, screenDashboard, m.selectedKeywordID)
	case "s":
		if m.selectedKeywordID != nil {
			m.loading = true
			m.statusMessage = "Running scan now..."
			return m, forceScanCmd(m.query, m.runtime, screenDashboard, *m.selectedKeywordID)
		}
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

func (m Model) currentDashboardDeals() []dto.GroupedListing {
	if m.dashboard == nil {
		return nil
	}
	return limitGroupedListings(m.dashboard.TopDeals, 5)
}

func (m Model) currentDetailDeals() []dto.GroupedListing {
	if m.detail == nil {
		return nil
	}
	return limitGroupedListings(m.detail.TopDeals, 5)
}

func (m Model) currentDashboardDealURL() (string, bool) {
	deals := m.currentDashboardDeals()
	if len(deals) == 0 {
		return "", false
	}
	idx := clampIndex(m.dashboardDealIndex, len(deals))
	if deals[idx].SampleURL == "" {
		return "", false
	}
	return deals[idx].SampleURL, true
}

func (m Model) currentDetailDealURL() (string, bool) {
	deals := m.currentDetailDeals()
	if len(deals) == 0 {
		return "", false
	}
	idx := clampIndex(m.detailDealIndex, len(deals))
	if deals[idx].SampleURL == "" {
		return "", false
	}
	return deals[idx].SampleURL, true
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("PriceAlert\n\nStatus: %s\n\nError: %v\n\nPress r to retry or q to quit.", m.statusMessage, m.err)
	}

	switch m.screen {
	case screenDetail:
		return renderDetail(m.detail, m.width, m.loading, m.statusMessage, m.editingField, m.editFieldInput, m.detailFocus, m.detailDealIndex)
	default:
		return renderDashboard(m.dashboard, m.width, m.selectedIndex, m.loading, m.statusMessage, m.addingKeyword, m.addKeywordInput, m.dashboardFocus, m.dashboardDealIndex)
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

func forceScanCmd(query QueryService, runtime RuntimeTrigger, target screen, keywordID string) tea.Cmd {
	return func() tea.Msg {
		if err := runtime.ScanNow(context.Background(), keywordID); err != nil {
			return forceScanDoneMsg{screen: target, err: err}
		}

		if target == screenDetail {
			detail, err := query.KeywordDetail(context.Background(), keywordID)
			status := "Scan complete"
			if detail != nil {
				status = fmt.Sprintf("Scan complete for %s", detail.Keyword.Keyword)
			}
			return forceScanDoneMsg{
				screen: screenDetail,
				detail: detail,
				status: status,
				err:    err,
			}
		}

		selectedID := keywordID
		state, err := query.DashboardState(context.Background(), &selectedID)
		status := "Scan complete"
		if state != nil && state.SelectedKeywordID != nil {
			for _, keyword := range state.TrackedKeywords {
				if keyword.ID == *state.SelectedKeywordID {
					status = fmt.Sprintf("Scan complete for %s", keyword.Keyword)
					break
				}
			}
		}
		return forceScanDoneMsg{
			screen: screenDashboard,
			state:  state,
			status: status,
			err:    err,
		}
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

func renderDashboard(state *dto.DashboardState, width int, selectedIndex int, loading bool, status string, adding bool, input string, focus dashboardFocus, dealIndex int) string {
	if state == nil {
		return fmt.Sprintf("PriceAlert\n\nStatus: %s", status)
	}

	layout := computeLayout(width)
	header := renderHeader("Dashboard", renderStatusLine(status, loading), layout.headerWidth)

	keywordLines := make([]string, 0, len(state.TrackedKeywords))
	if len(state.TrackedKeywords) == 0 {
		keywordLines = append(keywordLines, "(no tracked keywords)")
	} else {
		for index, keyword := range state.TrackedKeywords {
			prefix := "  "
			if index == selectedIndex {
				prefix = selectedBulletStyle.Render("● ")
			}
			alertMarker := ""
			if keyword.HasNewAlert {
				alertMarker = "  " + alertBadgeStyle.Render("ALERT")
			}
			keywordLines = append(keywordLines, fmt.Sprintf("%s%s", prefix, selectedKeywordText(index == selectedIndex, shorten(keyword.Keyword, layout.leftContentWidth-6))))
			keywordLines = append(keywordLines, fmt.Sprintf("  status %s%s", keyword.Status, alertMarker))
		}
	}

	snapshotLines := []string{"(no snapshot yet)"}
	if state.SelectedSnapshot == nil {
	} else {
		snapshotLines = []string{
			fmt.Sprintf("Signal: %s   At: %s", formatSignal(state.SelectedSnapshot.Signal), formatTimestamp(&state.SelectedSnapshot.SnapshotAt)),
			fmt.Sprintf("Min / Avg / Max: %s / %s / %s",
				formatPrice(state.SelectedSnapshot.MinPrice),
				formatPrice(state.SelectedSnapshot.AvgPrice),
				formatPrice(state.SelectedSnapshot.MaxPrice),
			),
			fmt.Sprintf("Raw / Grouped: %d / %d", state.SelectedSnapshot.RawCount, state.SelectedSnapshot.GroupedCount),
		}
	}

	dealLines := formatDashboardDeals(state.TopDeals, dealIndex, focus == dashboardFocusDeals, max(32, layout.rightContentWidth-12))
	eventLines := make([]string, 0, max(len(state.RecentEvents), 1))
	if len(state.RecentEvents) == 0 {
		eventLines = append(eventLines, "(no recent events)")
	}
	for _, event := range limitAlertEvents(state.RecentEvents, 4) {
		eventLines = append(eventLines, fmt.Sprintf("[%s] %s", event.EventType, shorten(event.Message, max(28, layout.rightContentWidth-14))))
	}

	left := lipgloss.JoinVertical(lipgloss.Left,
		renderPanel("Runtime", formatRuntimeStatus(state.RuntimeStatus), layout.leftWidth, false),
		"",
		renderPanel("Tracked Keywords", keywordLines, layout.leftWidth, focus == dashboardFocusKeywords),
	)

	rightItems := []string{
		renderPanel("Selected Snapshot", snapshotLines, layout.rightWidth, false),
		"",
		renderPanel("Top Deals", dealLines, layout.rightWidth, focus == dashboardFocusDeals),
	}
	if len(state.RecentEvents) > 0 {
		rightItems = append(rightItems, "", renderPanel("Recent Events", eventLines, layout.rightWidth, false))
	}
	if adding {
		rightItems = append([]string{
			renderPanel("Add Keyword", []string{
				fmt.Sprintf("Input: %s", input),
				"Enter to save, esc to cancel",
			}, layout.rightWidth, false),
			"",
		}, rightItems...)
	}
	right := lipgloss.JoinVertical(lipgloss.Left, rightItems...)
	body := joinColumns(layout, left, right)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		"",
		renderFooter("tab focus", "j/k move", "enter open/detail", "s scan-now", "a add", "p pause", "u resume", "x archive", "r refresh", "q quit"),
	)
}

func formatRuntimeStatus(status *dto.RuntimeStatusSummary) []string {
	if status == nil {
		return []string{"(runtime status unavailable)"}
	}

	accepting := "no"
	if status.AcceptingNewWork {
		accepting = "yes"
	}

	lines := []string{
		fmt.Sprintf("Accepting New Work: %s", accepting),
		fmt.Sprintf("Running: %d / %d", status.RunningCount, status.MaxConcurrent),
	}

	if status.ReconciledRunningJobs > 0 {
		lines = append(lines, fmt.Sprintf("Startup Reconciled: %d running job(s)", status.ReconciledRunningJobs))
	}
	if status.PrunedRawListings > 0 {
		lines = append(lines, fmt.Sprintf("Startup Pruned Raw Listings: %d", status.PrunedRawListings))
	}
	if status.PrunedAlertEvents > 0 {
		lines = append(lines, fmt.Sprintf("Startup Pruned Alert Events: %d", status.PrunedAlertEvents))
	}

	return lines
}

func renderDetail(detail *dto.KeywordDetail, width int, loading bool, status string, editingField editField, editInput string, focus detailFocus, dealIndex int) string {
	if detail == nil {
		return fmt.Sprintf("PriceAlert\n\nStatus: %s", status)
	}

	layout := computeLayout(width)
	header := renderHeader("Keyword Detail", renderStatusLine(status, loading), layout.headerWidth)

	keywordLines := []string{
		formatKeyValue("Keyword", shorten(detail.Keyword.Keyword, max(20, layout.leftContentWidth-12))),
		formatKeyValue("Status", detail.Keyword.Status),
		formatKeyValue("Interval", fmt.Sprintf("%d min", detail.Keyword.IntervalMinutes)),
		formatKeyValue("Threshold", formatPrice(detail.Keyword.ThresholdPrice)),
		formatKeyValue("Basic Filter", formatOptionalString(detail.Keyword.BasicFilter)),
		filterHintStyle.Render("Syntax: token token -exclude"),
		formatKeyValue("Telegram", fmt.Sprintf("%t", detail.Keyword.TelegramEnabled)),
	}
	if editingField != editFieldNone {
		keywordLines = append(keywordLines,
			formatKeyValue("Editing "+editingFieldLabel(editingField), editInput),
			"Enter to save, esc to cancel",
		)
	}

	snapshotLines := []string{"(no snapshot yet)"}
	if detail.Snapshot == nil {
	} else {
		snapshotLines = []string{
			formatKeyValue("Signal", formatSignal(detail.Snapshot.Signal)),
			formatKeyValue("Min / Avg / Max", fmt.Sprintf("%s / %s / %s",
				formatPrice(detail.Snapshot.MinPrice),
				formatPrice(detail.Snapshot.AvgPrice),
				formatPrice(detail.Snapshot.MaxPrice),
			)),
			formatKeyValue("Raw / Grouped", fmt.Sprintf("%d / %d", detail.Snapshot.RawCount, detail.Snapshot.GroupedCount)),
			formatKeyValue("At", formatTimestamp(&detail.Snapshot.SnapshotAt)),
		}
	}

	dealLines := formatDetailDeals(detail.TopDeals, dealIndex, focus == detailFocusDeals, max(30, layout.rightContentWidth-10))

	historyLines := []string{"(no price history)"}
	if len(detail.RecentHistory) == 0 {
	} else {
		historyLines = nil
		for _, point := range limitPricePoints(detail.RecentHistory, 5) {
			historyLines = append(historyLines, fmt.Sprintf("%s | %s / %s / %s",
				formatTimestamp(&point.RecordedAt),
				formatPrice(point.MinPrice),
				formatPrice(point.AvgPrice),
				formatPrice(point.MaxPrice),
			))
		}
	}

	eventLines := []string{"(no recent events)"}
	if len(detail.RecentEvents) == 0 {
	} else {
		eventLines = nil
		for _, event := range limitAlertEvents(detail.RecentEvents, 4) {
			eventLines = append(eventLines, fmt.Sprintf("[%s] %s", event.EventType, shorten(event.Message, max(28, layout.rightContentWidth-14))))
		}
	}

	left := lipgloss.JoinVertical(lipgloss.Left,
		renderPanel("Keyword", keywordLines, layout.leftWidth, false),
		"",
		renderPanel("Snapshot", snapshotLines, layout.leftWidth, false),
	)
	right := lipgloss.JoinVertical(lipgloss.Left,
		renderPanel("Top Deals", dealLines, layout.rightWidth, focus == detailFocusDeals),
		"",
		renderPanel("Recent History", historyLines, layout.rightWidth, focus == detailFocusHistory),
	)
	if len(detail.RecentEvents) > 0 {
		right = lipgloss.JoinVertical(lipgloss.Left, right, "", renderPanel("Recent Events", eventLines, layout.rightWidth, false))
	}
	body := joinColumns(layout, left, right)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		"",
		renderFooter("tab focus", "j/k move", "enter open", "s scan-now", "esc back", "p pause", "u resume", "x archive", "t telegram", "1 threshold", "2 interval", "3 filter", "r refresh", "q quit"),
	)
}

func renderStatusLine(status string, loading bool) string {
	if loading {
		return fmt.Sprintf("Status: %s  [loading]", status)
	}
	return fmt.Sprintf("Status: %s", status)
}

func renderHeader(screen, status string, width int) string {
	title := lipgloss.JoinHorizontal(lipgloss.Left,
		appTitleStyle.Render("PriceAlert"),
		" ",
		screenTitleStyle.Render(screen),
	)
	return lipgloss.JoinVertical(lipgloss.Left, title, headerBarStyle.Render(strings.Repeat("─", max(24, width))), statusStyle.Render(status))
}

func renderPanel(title string, lines []string, width int, focused bool) string {
	section := []string{panelTitleStyle.Render(title), ""}
	if len(lines) == 0 {
		section = append(section, "-")
		return panelBoxStyle(focused, width).Render(strings.Join(section, "\n"))
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		section = append(section, line)
	}
	return panelBoxStyle(focused, width).Render(strings.Join(section, "\n"))
}

func renderFooter(items ...string) string {
	return footerStyle.Render("Keys: " + strings.Join(items, " | "))
}

func formatKeyValue(label, value string) string {
	return keyLabelStyle.Render(label+":") + " " + value
}

func shorten(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func formatPrice(value *int64) string {
	if value == nil {
		return "-"
	}
	return formatInt64(*value)
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

var (
	appTitleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("25")).Padding(0, 1)
	screenTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	headerBarStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("61"))
	statusStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	panelStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("61")).Padding(0, 1)
	panelTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	footerStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	keyLabelStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Bold(true)
	metaStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	focusedMetaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("223"))
	filterHintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
	selectedBulletStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	selectedKeywordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)
	alertBadgeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160")).Padding(0, 1)
	buyNowSignalStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160")).Bold(true).Padding(0, 1)
	goodDealSignalStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(lipgloss.Color("214")).Bold(true).Padding(0, 1)
	normalSignalStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(lipgloss.Color("110")).Padding(0, 1)
	noDataSignalStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("238")).Padding(0, 1)
)

func selectedKeywordText(selected bool, value string) string {
	if !selected {
		return value
	}
	return selectedKeywordStyle.Render(value)
}

func formatSignal(signal string) string {
	switch signal {
	case "BUY_NOW":
		return buyNowSignalStyle.Render(signal)
	case "GOOD_DEAL":
		return goodDealSignalStyle.Render(signal)
	case "NORMAL":
		return normalSignalStyle.Render(signal)
	case "NO_DATA":
		return noDataSignalStyle.Render(signal)
	default:
		return signal
	}
}

func formatTimestamp(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func summarizeTopDeals(deals []dto.GroupedListing, limit int, titleWidth int) []string {
	if len(deals) == 0 {
		return []string{"(no grouped listings)"}
	}
	lines := make([]string, 0, limit*2)
	for _, deal := range limitGroupedListings(deals, limit) {
		lines = append(lines, shorten(deal.RepresentativeTitle, titleWidth))
		lines = append(lines, metaStyle.Render(fmt.Sprintf("%s  |  %s", formatPrice(&deal.BestPrice), shorten(deal.RepresentativeSeller, min(24, max(12, titleWidth/3))))))
		lines = append(lines, "")
	}
	return trimTrailingBlank(lines)
}

func summarizeDetailedDeals(deals []dto.GroupedListing, limit int, titleWidth int) []string {
	if len(deals) == 0 {
		return []string{"(no grouped listings)"}
	}
	lines := make([]string, 0, limit*2)
	for _, deal := range limitGroupedListings(deals, limit) {
		lines = append(lines, shorten(deal.RepresentativeTitle, titleWidth))
		lines = append(lines, metaStyle.Render(fmt.Sprintf("%s  |  %s  |  %d listing(s)", shorten(deal.RepresentativeSeller, min(20, max(10, titleWidth/4))), formatPrice(&deal.BestPrice), deal.ListingCount)))
		lines = append(lines, "")
	}
	return trimTrailingBlank(lines)
}

func formatDashboardDeals(deals []dto.GroupedListing, selectedIndex int, focused bool, titleWidth int) []string {
	return formatDealLines(limitGroupedListings(deals, 5), selectedIndex, focused, titleWidth, false)
}

func formatDetailDeals(deals []dto.GroupedListing, selectedIndex int, focused bool, titleWidth int) []string {
	return formatDealLines(limitGroupedListings(deals, 5), selectedIndex, focused, titleWidth, true)
}

func formatDealLines(deals []dto.GroupedListing, selectedIndex int, focused bool, titleWidth int, includeCount bool) []string {
	if len(deals) == 0 {
		return []string{"(no grouped listings)"}
	}
	lines := make([]string, 0, len(deals)*3)
	for i, deal := range deals {
		prefix := "  "
		title := shorten(deal.RepresentativeTitle, titleWidth)
		meta := fmt.Sprintf("%s  |  %s", formatPrice(&deal.BestPrice), shorten(deal.RepresentativeSeller, min(24, max(12, titleWidth/3))))
		if includeCount {
			meta = fmt.Sprintf("%s  |  %s  |  %d listing(s)", shorten(deal.RepresentativeSeller, min(20, max(10, titleWidth/4))), formatPrice(&deal.BestPrice), deal.ListingCount)
		}
		if i == selectedIndex {
			prefix = selectedBulletStyle.Render("▶ ")
			if focused {
				title = selectedKeywordStyle.Render(title)
				meta = focusedMetaStyle.Render(meta)
			}
		}
		lines = append(lines, prefix+title)
		lines = append(lines, "  "+meta)
		lines = append(lines, "")
	}
	return trimTrailingBlank(lines)
}

func limitGroupedListings(items []dto.GroupedListing, limit int) []dto.GroupedListing {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitAlertEvents(items []dto.AlertEvent, limit int) []dto.AlertEvent {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitPricePoints(items []dto.PricePoint, limit int) []dto.PricePoint {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func formatInt64(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	raw := strconv.FormatInt(value, 10)
	if len(raw) <= 3 {
		return sign + raw
	}
	parts := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return sign + strings.Join(parts, ".")
}

type layoutDimensions struct {
	stacked           bool
	headerWidth       int
	leftWidth         int
	rightWidth        int
	leftContentWidth  int
	rightContentWidth int
}

func computeLayout(width int) layoutDimensions {
	if width <= 0 {
		width = 120
	}
	contentWidth := max(72, width-4)
	if contentWidth < 112 {
		panelWidth := contentWidth - 2
		return layoutDimensions{
			stacked:           true,
			headerWidth:       contentWidth,
			leftWidth:         panelWidth,
			rightWidth:        panelWidth,
			leftContentWidth:  panelWidth - 4,
			rightContentWidth: panelWidth - 4,
		}
	}

	leftWidth := max(30, min(42, contentWidth/3))
	rightWidth := max(40, contentWidth-leftWidth-2)
	return layoutDimensions{
		headerWidth:       contentWidth,
		leftWidth:         leftWidth,
		rightWidth:        rightWidth,
		leftContentWidth:  leftWidth - 4,
		rightContentWidth: rightWidth - 4,
	}
}

func joinColumns(layout layoutDimensions, left string, right string) string {
	if layout.stacked {
		return lipgloss.JoinVertical(lipgloss.Left, left, "", right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func nextDashboardFocus(current dashboardFocus) dashboardFocus {
	switch current {
	case dashboardFocusDeals:
		return dashboardFocusKeywords
	default:
		return dashboardFocusDeals
	}
}

func nextDetailFocus(current detailFocus) detailFocus {
	switch current {
	case detailFocusHistory:
		return detailFocusDeals
	default:
		return detailFocusHistory
	}
}

func clampIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func panelBoxStyle(focused bool, width int) lipgloss.Style {
	style := panelStyle.Width(max(20, width))
	if focused {
		return style.BorderForeground(lipgloss.Color("86"))
	}
	return style
}

func openURLCmd(opener BrowserOpener, url string) tea.Cmd {
	return func() tea.Msg {
		if opener == nil {
			opener = systemBrowserOpener{}
		}
		return openURLDoneMsg{
			url: url,
			err: opener.OpenURL(context.Background(), url),
		}
	}
}

type systemBrowserOpener struct{}

func (systemBrowserOpener) OpenURL(ctx context.Context, url string) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("empty url")
	}
	return exec.CommandContext(ctx, "xdg-open", url).Start()
}
