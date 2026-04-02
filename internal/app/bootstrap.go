package app

import "github.com/pricealert/pricealert/internal/config"

// App is a thin bootstrap shell for milestone A.
type App struct {
	cfg config.Config
}

func New(cfg config.Config) (*App, error) {
	return &App{cfg: cfg}, nil
}

func (a *App) Run() error {
	// Runtime/TUI/worker wiring is intentionally deferred to later milestones.
	_ = a.cfg
	return nil
}
