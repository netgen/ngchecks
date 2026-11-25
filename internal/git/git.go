// Package git runs the git commands ngcheck needs.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository is a git work tree.
type Repository struct {
	Root string
}

// Open returns the repository containing dir.
func Open(dir string) (*Repository, error) {
	output, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%q is not inside a git repository", dir)
	}

	return &Repository{Root: strings.TrimSpace(output)}, nil
}

// ResolveCommit returns the full SHA of a ref, or false if it does not exist locally.
func (r *Repository) ResolveCommit(ref string) (string, bool) {
	if strings.HasPrefix(ref, "-") {
		return "", false
	}

	output, err := run(r.Root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", false
	}

	return strings.TrimSpace(output), true
}

// MergeBase returns the best common ancestor of two commits.
func (r *Repository) MergeBase(commit, otherCommit string) (string, bool) {
	output, err := run(r.Root, "merge-base", commit, otherCommit)
	if err != nil {
		return "", false
	}

	return strings.TrimSpace(output), true
}

// FetchBranch fetches a branch from origin into refs/remotes/origin/<branch>.
func (r *Repository) FetchBranch(branch string) bool {
	if strings.HasPrefix(branch, "-") {
		return false
	}

	_, err := run(r.Root, "fetch", "--no-tags", "origin", fmt.Sprintf("+refs/heads/%[1]s:refs/remotes/origin/%[1]s", branch))

	return err == nil
}

// IsShallow reports whether the repository is a shallow clone.
func (r *Repository) IsShallow() bool {
	output, err := run(r.Root, "rev-parse", "--is-shallow-repository")

	return err == nil && strings.TrimSpace(output) == "true"
}

// Unshallow fetches the complete history from origin.
func (r *Repository) Unshallow() bool {
	_, err := run(r.Root, "fetch", "--unshallow", "--no-tags", "origin")

	return err == nil
}

// Config returns a git config value, or "" if it is not set.
func (r *Repository) Config(key string) string {
	output, err := run(r.Root, "config", "--get", key)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}

// HooksDir returns the absolute path of the directory git runs hooks from.
func (r *Repository) HooksDir() (string, error) {
	output, err := run(r.Root, "rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", err
	}

	path := strings.TrimSpace(output)
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.Root, path)
	}

	return path, nil
}

// RemoteURL returns the URL of a remote, or "" if it does not exist.
func (r *Repository) RemoteURL(name string) string {
	output, err := run(r.Root, "remote", "get-url", name)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}

// TrackedFiles returns the paths (slash separated, relative to the root) of
// regular files tracked by git. Symlinks and submodules are skipped.
func (r *Repository) TrackedFiles() ([]string, error) {
	output, err := run(r.Root, "ls-files", "-z", "--stage")
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var files []string

	for _, entry := range strings.Split(output, "\x00") {
		// "<mode> <object> <stage>\t<path>"
		meta, path, found := strings.Cut(entry, "\t")
		if !found || seen[path] || !(strings.HasPrefix(meta, "100644 ") || strings.HasPrefix(meta, "100755 ")) {
			continue
		}

		seen[path] = true
		files = append(files, path)
	}

	return files, nil
}

// Diff returns the unified diff without context lines between two commits.
func (r *Repository) Diff(from, to string) (string, error) {
	return run(r.Root, "-c", "core.quotePath=false", "diff", "--unified=0", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames", from, to)
}

func run(dir string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(stderr.String()))
		}

		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	return stdout.String(), nil
}
