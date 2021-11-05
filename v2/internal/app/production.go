//go:build production
// +build production

package app

import "github.com/sergey-shpilevskiy/wails/v2/pkg/logger"

// Init initialises the application for a production environment
func (a *App) Init() error {
	a.logger.SetLogLevel(logger.ERROR)
	return nil
}
