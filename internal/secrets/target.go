// Package secrets scans git repositories for leaked secrets with Betterleaks.
package secrets

import "fmt"

// Mode selects what part of a repository is scanned.
type Mode int

const (
	// StagedChanges are the changes staged for the next commit (pre-commit hook).
	StagedChanges Mode = iota
	// TrackedFiles is the current content of all files tracked by git.
	TrackedFiles
	// CommitRange is the commits reachable from HEAD but not from Target.Base.
	CommitRange
	// History is every commit of every ref.
	History
)

// Target is what a scan covers.
type Target struct {
	Mode Mode
	// Base is the excluded commit of a CommitRange (Base..HEAD).
	Base string
}

// ScansCommits reports whether findings carry commit information.
func (t Target) ScansCommits() bool {
	return t.Mode == CommitRange || t.Mode == History
}

// Describe returns a human readable description, e.g. "commits 1a2b3c4d5e6f..HEAD".
func (t Target) Describe() string {
	switch t.Mode {
	case StagedChanges:
		return "staged changes"
	case TrackedFiles:
		return "tracked files"
	case CommitRange:
		return fmt.Sprintf("commits %.12s..HEAD", t.Base)
	default:
		return "full git history"
	}
}
