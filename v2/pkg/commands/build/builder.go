package build

import (
	"github.com/sergey-shpilevskiy/wails/v2/internal/project"
	"github.com/sergey-shpilevskiy/wails/v2/pkg/clilogger"
)

// Builder defines a builder that can build Wails applications
type Builder interface {
	SetProjectData(projectData *project.Project)
	BuildAssets(*Options) error
	BuildFrontend(*clilogger.CLILogger) error
	BuildRuntime(*Options) error
	CompileProject(*Options) error
	OutputFilename(*Options) string
	PostCompilation(*Options) error
	CleanUp()
}
