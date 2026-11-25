package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/netgen/ngcheck/internal/betterleaks"
	"github.com/netgen/ngcheck/internal/cli"
	"github.com/netgen/ngcheck/internal/testutil"
)

type result struct {
	exitCode       int
	stdout, stderr string
}

// run executes ngcheck with only the given environment variables (plus the Betterleaks binary).
func run(t *testing.T, environment map[string]string, args ...string) result {
	t.Helper()

	binary := testutil.Betterleaks(t)
	getenv := func(name string) string {
		if name == betterleaks.EnvBinary {
			return binary
		}

		return environment[name]
	}

	var stdout, stderr bytes.Buffer
	exitCode := cli.Run(args, &stdout, &stderr, getenv, "1.2.3")

	return result{exitCode, stdout.String(), stderr.String()}
}

func assertContains(t *testing.T, haystack string, needles ...string) {
	t.Helper()

	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			t.Errorf("expected output to contain %q:\n%s", needle, haystack)
		}
	}
}

func leak() string {
	return "<?php return '" + testutil.FakeGitHubToken() + "';\n"
}

func TestReportsLeakInTrackedFilesAndWritesReports(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("src/client.php", "<?php\n"+leak()).Commit("Leak")

	r := run(t, nil, "secrets", repo.Root, "--junit", repo.Root+"/reports/secrets.xml", "--json", repo.Root+"/reports/secrets.json")

	if r.exitCode != cli.ExitFindings {
		t.Fatalf("expected exit code 1, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}
	assertContains(t, r.stdout, "Scanning tracked files in "+repo.Root+"\n", "github-pat  src/client.php:2", "1 potential secret(s) found")

	junit, _ := os.ReadFile(filepath.Join(repo.Root, "reports/secrets.xml"))
	assertContains(t, string(junit), `<testcase classname="github-pat" name="src/client.php:2"`)

	json, _ := os.ReadFile(filepath.Join(repo.Root, "reports/secrets.json"))
	assertContains(t, string(json), `"fingerprint": "src/client.php:github-pat:2"`)
}

func TestSucceedsWithoutLeaks(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("README.md", "# Clean\n").Commit("Clean")

	r := run(t, nil, "secrets", repo.Root)

	if r.exitCode != cli.ExitOK {
		t.Fatalf("expected exit code 0, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}
	assertContains(t, r.stdout, "OK: no secrets found.")
}

func TestStagedScanFromSubdirectory(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("README.md", "# Test\n").Commit("Initial commit")
	repo.Write("src/client.php", leak()).Git("add", "src/client.php")

	r := run(t, nil, "secrets", "--staged", filepath.Join(repo.Root, "src"))

	if r.exitCode != cli.ExitFindings {
		t.Fatalf("expected exit code 1, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}
	assertContains(t, r.stdout, "Scanning staged changes", "src/client.php:1")
}

func TestWriteIgnoreAcceptsCurrentFindings(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("config.php", leak()).Commit("Leak")

	if r := run(t, nil, "secrets", "--write-ignore", repo.Root); r.exitCode != cli.ExitOK {
		t.Fatalf("expected exit code 0, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}

	ignore, _ := os.ReadFile(filepath.Join(repo.Root, ".betterleaksignore"))
	assertContains(t, string(ignore), "# Accepted with ngcheck secrets --write-ignore on ", "\nconfig.php:github-pat:1\n")

	if r := run(t, nil, "secrets", repo.Root); r.exitCode != cli.ExitOK {
		t.Errorf("expected exit code 0 after accepting, got %d\n%s", r.exitCode, r.stdout)
	}
}

func TestScansPullRequestCommitsOnGitHub(t *testing.T) {
	origin := testutil.NewRepo(t)
	origin.Write("old.php", leak()).Commit("Old leak on master")

	clone := origin.Clone()
	clone.Git("checkout", "--quiet", "-b", "feature")
	clone.Write("leak.php", leak()).
		Write(".betterleaksignore", "abc123:other.php:generic-api-key:1\n").
		Commit("Feature")
	// A single-branch CI checkout has no base ref: ngcheck fetches it.
	clone.Git("update-ref", "-d", "refs/remotes/origin/master")

	r := run(t, map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_BASE_REF": "master"}, "secrets", clone.Root)

	if r.exitCode != cli.ExitFindings {
		t.Fatalf("expected exit code 1, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}
	assertContains(t, r.stdout,
		"(GitHub Actions)",
		"::error file=leak.php,line=1,title=Potential secret%3A github-pat::",
		"::warning file=.betterleaksignore,line=1,title=Secret scan suppression::ignore file entry: abc123",
	)
	if strings.Contains(r.stdout, "old.php") {
		t.Errorf("commits of the base branch must not be scanned:\n%s", r.stdout)
	}
}

func TestScansMergeRequestCommitsOnGitLab(t *testing.T) {
	repo := testutil.NewRepo(t)
	base := repo.Write("old.php", leak()).Commit("Old leak on main")
	repo.Git("checkout", "--quiet", "-b", "feature")
	repo.Write("leak.php", leak()).Commit("Feature")

	r := run(t, map[string]string{"GITLAB_CI": "true", "CI_MERGE_REQUEST_DIFF_BASE_SHA": base}, "secrets", repo.Root)

	if r.exitCode != cli.ExitFindings {
		t.Fatalf("expected exit code 1, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}
	assertContains(t, r.stdout, "Scanning commits "+base[:12]+"..HEAD", "(GitLab CI)", "leak.php:1")
	if strings.Contains(r.stdout, "old.php") || strings.Contains(r.stdout, "::error") {
		t.Errorf("unexpected output:\n%s", r.stdout)
	}
}

func TestFailsWhenPullRequestBaseIsUnknown(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("README.md", "# Test\n").Commit("Initial commit")

	r := run(t, map[string]string{"BITBUCKET_BUILD_NUMBER": "1", "BITBUCKET_PR_DESTINATION_BRANCH": "develop"}, "secrets", repo.Root)

	if r.exitCode != cli.ExitError {
		t.Fatalf("expected exit code 2, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}
	assertContains(t, r.stderr, `cannot find the pull request base "origin/develop"`)
}

func TestRejectsInvalidUsage(t *testing.T) {
	repo := testutil.NewRepo(t)

	tests := map[string][]string{
		"multiple modes":           {"secrets", "--staged", "--history", repo.Root},
		"write-ignore with staged": {"secrets", "--staged", "--write-ignore", repo.Root},
		"two paths":                {"secrets", repo.Root, repo.Root},
		"not a repository":         {"secrets", t.TempDir()},
		"unknown command":          {"lint"},
		"no command":               {},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			if r := run(t, nil, args...); r.exitCode != cli.ExitError {
				t.Errorf("expected exit code 2, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	r := run(t, nil, "version")

	if r.exitCode != cli.ExitOK {
		t.Fatalf("expected exit code 0, got %d\n%s", r.exitCode, r.stderr)
	}
	assertContains(t, r.stdout, "ngcheck 1.2.3\nbetterleaks ")
}

func TestInit(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write(".husky/pre-commit", "npm test\n").Write(".github/workflows/ci.yml", "name: CI\n")

	r := run(t, nil, "init", repo.Root)

	if r.exitCode != cli.ExitOK {
		t.Fatalf("expected exit code 0, got %d\n%s%s", r.exitCode, r.stdout, r.stderr)
	}
	assertContains(t, r.stdout,
		"Setting up ngcheck in "+repo.Root,
		`changed  .husky/pre-commit: added "ngcheck secrets --staged"`,
		"changed  .github/workflows/secrets.yml: created the pull request check",
		"todo     Husky is not active in this clone",
		"\n    npx husky\n",
		"git add .husky/pre-commit .github/workflows/secrets.yml",
	)

	r = run(t, nil, "init", repo.Root)
	assertContains(t, r.stdout, "ok       .husky/pre-commit: already runs ngcheck")
	if strings.Contains(r.stdout, "git add") {
		t.Errorf("nothing should change on the second run:\n%s", r.stdout)
	}
}
