package app

import (
	"github.com/charmbracelet/bubbletea"

	"github.com/pricealert/pricealert/internal/service/query"
	"github.com/pricealert/pricealert/internal/tui"
)

func newQueryService(repos appRepositories) *query.Service {
	return query.NewService(
		repos.trackedKeywords,
		repos.groupedListings,
		repos.snapshots,
		repos.pricePoints,
		repos.alertEvents,
	)
}

func newTUIProgram(queries *query.Service) *tea.Program {
	model := tui.NewModel(queries)
	return tea.NewProgram(model)
}
