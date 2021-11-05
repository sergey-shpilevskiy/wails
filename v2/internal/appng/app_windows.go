//go:build windows && !bindings

package appng

import (
	"github.com/sergey-shpilevskiy/wails/v2/internal/ffenestri/windows/wv2runtime"
	"github.com/sergey-shpilevskiy/wails/v2/internal/logger"
	"github.com/sergey-shpilevskiy/wails/v2/pkg/options"
)

func PreflightChecks(options *options.App, logger *logger.Logger) error {

	_ = options

	// Process the webview2 runtime situation. We can pass a strategy in via the `webview2` flag for `wails build`.
	// This will determine how wv2runtime.Process will handle a lack of valid runtime.
	installedVersion, err := wv2runtime.Process()
	if installedVersion != nil {
		logger.Debug("WebView2 Runtime installed: Name: '%s' Version:'%s' Location:'%s'. Minimum version required: %s.",
			installedVersion.Name, installedVersion.Version, installedVersion.Location, wv2runtime.MinimumRuntimeVersion)
	}
	if err != nil {
		return err
	}

	return nil
}
