package helpers

import (
	"os"
	"path/filepath"

	"github.com/kardianos/service"
	"github.com/labstack/gommon/log"
)

func RelativePath(path string) string {
	if service.Interactive() {
		return path
	}

	// For future usage when running as a service
	exePath, err := os.Executable()
	if err != nil {
		log.Panicf("Failed to get executable path: %s", err)
	}

	return filepath.Join(filepath.Dir(exePath), path)
}
