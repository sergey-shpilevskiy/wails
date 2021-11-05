//go:build darwin && !bindings

package appng

import (
	"github.com/sergey-shpilevskiy/wails/v2/internal/logger"
	"github.com/sergey-shpilevskiy/wails/v2/pkg/options"
)

func PreflightChecks(options *options.App, logger *logger.Logger) error {

	_ = options

	return nil
}
