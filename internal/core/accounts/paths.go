package accounts

import (
	"os"
	"path/filepath"
	"runtime"
)

const accountFileName = "accounts.enc"

func DefaultStorePath() (string, error) {
	base, err := configBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "utils", accountFileName), nil
}

func configBaseDir() (string, error) {
	if runtime.GOOS == osWindows {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return localAppData, nil
		}
		return os.UserConfigDir()
	}
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return xdgConfigHome, nil
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".config"), nil
	}
	return os.UserConfigDir()
}
