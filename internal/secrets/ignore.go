package secrets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AcceptFindings appends the fingerprints of findings that are not ignored yet to the
// repository's ignore file, preceded by a comment. It returns how many were added.
func AcceptFindings(repositoryRoot string, findings []Finding, comment string) (int, error) {
	path := filepath.Join(repositoryRoot, IgnoreFile)

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}

	known := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		known[strings.TrimSpace(line)] = true
	}

	var added []string
	for _, finding := range findings {
		if !known[finding.Fingerprint] {
			known[finding.Fingerprint] = true
			added = append(added, finding.Fingerprint)
		}
	}

	if len(added) == 0 {
		return 0, nil
	}

	content := strings.TrimRight(string(existing), "\n")
	if content != "" {
		content += "\n\n"
	}
	content += fmt.Sprintf("# %s\n%s\n", comment, strings.Join(added, "\n"))

	return len(added), os.WriteFile(path, []byte(content), 0o644)
}
