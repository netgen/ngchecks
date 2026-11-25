package secrets

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/netgen/ngchecks/internal/git"
)

// Scanner runs Betterleaks with the built-in ruleset, the repository's own config
// (.betterleaks.toml) and its ignore file (.betterleaksignore).
type Scanner struct {
	// Binary is the Betterleaks executable.
	Binary string
	// Ruleset is the built-in ruleset (TOML) the project config extends.
	Ruleset []byte
}

// Scan reports potential secrets in the target part of the repository.
func (s Scanner) Scan(repositoryRoot string, target Target) ([]Finding, error) {
	workDir, err := os.MkdirTemp("", "ngchecks-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	config, err := composeConfig(repositoryRoot, s.Ruleset, workDir)
	if err != nil {
		return nil, err
	}

	dir := repositoryRoot
	var args []string

	switch target.Mode {
	case StagedChanges:
		args = []string{"git", "--pre-commit", "--staged", repositoryRoot}
	case CommitRange:
		args = []string{"git", "--log-opts=" + target.Base + "..HEAD", repositoryRoot}
	case History:
		args = []string{"git", repositoryRoot}
	case TrackedFiles:
		dir = filepath.Join(workDir, "files")
		if err := copyTrackedFiles(repositoryRoot, dir); err != nil {
			return nil, err
		}
		args = []string{"dir", "."}
	}

	report := filepath.Join(workDir, "report.json")
	args = append(args,
		"--config", config,
		"--gitleaks-ignore-path", repositoryRoot,
		"--report-format", "json",
		"--report-path", report,
		"--exit-code", "0",
		"--redact",
		"--no-banner",
		"--no-color",
		"--log-level", "error",
	)

	command := exec.Command(s.Binary, args...)
	command.Dir = dir

	if output, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("betterleaks failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return readReport(report)
}

// copyTrackedFiles copies the working tree version of every tracked file, so untracked
// and ignored files (vendor/, node_modules/, .env.local, var/...) are not scanned.
func copyTrackedFiles(repositoryRoot, destination string) error {
	repository := &git.Repository{Root: repositoryRoot}

	files, err := repository.TrackedFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		source := filepath.Join(repositoryRoot, filepath.FromSlash(file))

		info, err := os.Lstat(source)
		if err != nil || !info.Mode().IsRegular() {
			// Deleted in the working tree, or replaced by something else.
			continue
		}

		if err := copyFile(source, filepath.Join(destination, filepath.FromSlash(file))); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}
