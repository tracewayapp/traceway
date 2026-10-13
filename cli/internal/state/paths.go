package state

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
)

// statePath returns the path to the state file, resolving XDG_STATE_HOME
// or falling back to $HOME/.local/state. It does not create the file.
func statePath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, appdir.Name, "state.json"), nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("neither XDG_STATE_HOME nor HOME is set")
	}
	return filepath.Join(home, ".local", "state", appdir.Name, "state.json"), nil
}
