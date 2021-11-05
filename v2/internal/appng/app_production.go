//go:build production
// +build production

package appng

import (
	"context"
	"github.com/sergey-shpilevskiy/wails/v2/internal/binding"
	"github.com/sergey-shpilevskiy/wails/v2/internal/frontend"
	"github.com/sergey-shpilevskiy/wails/v2/internal/frontend/desktop"
	"github.com/sergey-shpilevskiy/wails/v2/internal/frontend/dispatcher"
	"github.com/sergey-shpilevskiy/wails/v2/internal/frontend/runtime"
	"github.com/sergey-shpilevskiy/wails/v2/internal/logger"
	"github.com/sergey-shpilevskiy/wails/v2/internal/menumanager"
	"github.com/sergey-shpilevskiy/wails/v2/internal/signal"
	"github.com/sergey-shpilevskiy/wails/v2/pkg/options"
)

// App defines a Wails application structure
type App struct {
	frontend frontend.Frontend
	logger   *logger.Logger
	signal   *signal.Manager
	options  *options.App

	menuManager *menumanager.Manager

	// Indicates if the app is in debug mode
	debug bool

	// OnStartup/OnShutdown
	startupCallback  func(ctx context.Context)
	shutdownCallback func(ctx context.Context)
	ctx              context.Context
}

func (a *App) Run() error {
	err := a.frontend.Run(a.ctx)
	if a.shutdownCallback != nil {
		a.shutdownCallback(a.ctx)
	}
	return err
}

// CreateApp creates the app!
func CreateApp(appoptions *options.App) (*App, error) {
	var err error

	ctx := context.Background()

	// Merge default options
	options.MergeDefaults(appoptions)

	// Set up logger
	myLogger := logger.New(appoptions.Logger)
	myLogger.SetLogLevel(appoptions.LogLevel)

	// Preflight Checks
	err = PreflightChecks(appoptions, myLogger)
	if err != nil {
		return nil, err
	}

	// Create the menu manager
	menuManager := menumanager.NewManager()

	// Process the application menu
	if appoptions.Menu != nil {
		err = menuManager.SetApplicationMenu(appoptions.Menu)
		if err != nil {
			return nil, err
		}
	}

	// Create binding exemptions - Ugly hack. There must be a better way
	bindingExemptions := []interface{}{appoptions.OnStartup, appoptions.OnShutdown, appoptions.OnDomReady}
	appBindings := binding.NewBindings(myLogger, appoptions.Bind, bindingExemptions)
	eventHandler := runtime.NewEvents(myLogger)
	ctx = context.WithValue(ctx, "events", eventHandler)
	messageDispatcher := dispatcher.NewDispatcher(myLogger, appBindings, eventHandler)

	appFrontend := desktop.NewFrontend(ctx, appoptions, myLogger, appBindings, messageDispatcher)
	eventHandler.AddFrontend(appFrontend)

	result := &App{
		ctx:              ctx,
		frontend:         appFrontend,
		logger:           myLogger,
		menuManager:      menuManager,
		startupCallback:  appoptions.OnStartup,
		shutdownCallback: appoptions.OnShutdown,
		debug:            false,
	}

	result.options = appoptions

	result.ctx = context.WithValue(result.ctx, "debug", result.debug)

	return result, nil

}
