//go:build !dev && !production && !bindings && darwin

package appng

import (
	"fmt"

	"github.com/sergey-shpilevskiy/wails/v2/pkg/options"
)

// App defines a Wails application structure
type App struct{}

func (a *App) Run() error {
	return nil
}

// CreateApp creates the app!
func CreateApp(_ *options.App) (*App, error) {
	//	result := w32.MessageBox(0,
	//		`Wails applications will not build without the correct build tags.
	//Please use "wails build" or press "OK" to open the documentation on how to use "go build"`,
	//		"Error",
	//		w32.MB_ICONERROR|w32.MB_OKCANCEL)
	//	if result == 1 {
	//		exec.Command("rundll32", "url.dll,FileProtocolHandler", "https://wails.io").Start()
	//	}

	err := fmt.Errorf(`Wails applications will not build without the correct build tags.`)

	return nil, err
}
