package git

import (
	"runtime"

	"github.com/sergey-shpilevskiy/wails/v2/internal/shell"
)

func gitcommand() string {
	gitcommand := "git"
	if runtime.GOOS == "windows" {
		gitcommand = "git.exe"
	}

	return gitcommand
}

// IsInstalled returns true if git is installed for the given platform
func IsInstalled() bool {
	return shell.CommandExists(gitcommand())
}

// Email tries to retrieve the
func Email() (string, error) {
	stdout, _, err := shell.RunCommand(".", gitcommand(), "config", "user.email")
	return stdout, err
}

// Name tries to retrieve the
func Name() (string, error) {
	stdout, _, err := shell.RunCommand(".", gitcommand(), "config", "user.name")
	return stdout, err
}

func InitRepo(projectDir string) error {
	_, _, err := shell.RunCommand(projectDir, gitcommand(), "init")
	return err
}
