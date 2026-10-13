package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/tracewayapp/traceway/cli/internal/appdir"
)

// configPath returns the path to the config file, resolving XDG_CONFIG_HOME
// or falling back to $HOME/.config. It does not create the file.
func configPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, appdir.Name, "config.json"), nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("neither XDG_CONFIG_HOME nor HOME is set")
	}
	return filepath.Join(home, ".config", appdir.Name, "config.json"), nil
}
