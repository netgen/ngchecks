// Package testutil provides throwaway git repositories and fake secrets for tests.
package testutil

import (
	"crypto/rand"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/netgen/ngchecks/internal/betterleaks"
)

// Betterleaks returns the Betterleaks binary for integration tests ($NGCHECKS_BETTERLEAKS,
// set by "make test").
func Betterleaks(t *testing.T) string {
	t.Helper()

	binary := os.Getenv(betterleaks.EnvBinary)
	if binary == "" {
		t.Fatalf("%s is not set, run the tests with \"make test\"", betterleaks.EnvBinary)
	}

	return binary
}

// FakeGitHubToken returns a random token in GitHub personal access token format.
func FakeGitHubToken() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	var token strings.Builder
	token.WriteString("ghp_")

	for range 36 {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			panic(err)
		}
		token.WriteByte(alphabet[index.Int64()])
	}

	return token.String()
}

// WriteFile writes a file, failing the test on error.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Repo is a git repository in a temporary directory, removed after the test.
type Repo struct {
	t    *testing.T
	Root string
}

// NewRepo initializes an empty repository on branch master.
func NewRepo(t *testing.T) *Repo {
	t.Helper()

	repo := &Repo{t: t, Root: t.TempDir()}
	repo.Git("init", "--quiet", "--initial-branch=master")
	repo.configure()

	return repo
}

// NewBareRepo initializes a bare repository, usable as a remote.
func NewBareRepo(t *testing.T) *Repo {
	t.Helper()

	repo := &Repo{t: t, Root: t.TempDir()}
	repo.Git("init", "--quiet", "--bare", "--initial-branch=master")

	return repo
}

// Clone clones the repository into a new temporary directory.
func (r *Repo) Clone() *Repo {
	r.t.Helper()

	clone := &Repo{t: r.t, Root: filepath.Join(r.t.TempDir(), "clone")}
	run(r.t, "", "git", "clone", "--quiet", r.Root, clone.Root)
	clone.configure()

	return clone
}

// Write creates or overwrites a file (slash separated path relative to the root).
func (r *Repo) Write(file, content string) *Repo {
	r.t.Helper()

	WriteFile(r.t, filepath.Join(r.Root, filepath.FromSlash(file)), content)

	return r
}

// Delete removes a file from the working tree.
func (r *Repo) Delete(file string) *Repo {
	r.t.Helper()

	if err := os.Remove(filepath.Join(r.Root, filepath.FromSlash(file))); err != nil {
		r.t.Fatal(err)
	}

	return r
}

// Commit stages everything and commits, returning the commit SHA.
func (r *Repo) Commit(message string) string {
	r.t.Helper()

	r.Git("add", "--all")
	r.Git("commit", "--quiet", "--allow-empty", "--message", message)

	return r.Git("rev-parse", "HEAD")
}

// Git runs a git command in the repository and returns its trimmed output.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()

	return run(r.t, r.Root, "git", args...)
}

func (r *Repo) configure() {
	r.Git("config", "user.name", "Test User")
	r.Git("config", "user.email", "test@example.com")
	r.Git("config", "commit.gpgsign", "false")
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()

	command := exec.Command(name, args...)
	command.Dir = dir

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}

	return strings.TrimSpace(string(output))
}
