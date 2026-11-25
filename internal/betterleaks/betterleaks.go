// Package betterleaks locates the Betterleaks binary bundled with ngchecks.
package betterleaks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnvBinary overrides the Betterleaks binary location, e.g. for development and tests.
const EnvBinary = "NGCHECKS_BETTERLEAKS"

// Locate returns the Betterleaks binary: $NGCHECKS_BETTERLEAKS, or the "betterleaks"
// file next to the ngchecks executable, as shipped in every ngchecks distribution.
// PATH is deliberately not searched: the ruleset is tested against one pinned version.
func Locate(getenv func(string) string) (string, error) {
	if path := getenv(EnvBinary); path != "" {
		return path, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate the ngchecks executable: %w", err)
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	path := filepath.Join(filepath.Dir(executable), "betterleaks")

	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("betterleaks binary not found at %s (it ships next to ngchecks; set %s to use another one)", path, EnvBinary)
	}

	return path, nil
}

// Version returns the version reported by the binary, e.g. "1.10.1".
func Version(binary string) (string, error) {
	output, err := exec.Command(binary, "version").Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return "", fmt.Errorf("%s version: %w: %s", binary, err, strings.TrimSpace(string(exitError.Stderr)))
		}

		return "", fmt.Errorf("%s version: %w", binary, err)
	}

	return strings.TrimSpace(string(output)), nil
}
