package secrets_test

import (
	"slices"
	"testing"

	"github.com/netgen/ngcheck/internal/secrets"
	"github.com/netgen/ngcheck/internal/testutil"
	"github.com/netgen/ngcheck/ruleset"
)

func scan(t *testing.T, repo *testutil.Repo, target secrets.Target) []secrets.Finding {
	t.Helper()

	findings, err := secrets.Scanner{Binary: testutil.Betterleaks(t), Ruleset: ruleset.TOML}.Scan(repo.Root, target)
	if err != nil {
		t.Fatal(err)
	}

	return findings
}

func locations(findings []secrets.Finding) []string {
	result := []string{}
	for _, finding := range findings {
		result = append(result, finding.Location())
	}

	return result
}

func newRepo(t *testing.T) *testutil.Repo {
	repo := testutil.NewRepo(t)
	repo.Write("README.md", "# Test\n").Commit("Initial commit")

	return repo
}

func token() string {
	return "'" + testutil.FakeGitHubToken() + "'"
}

func TestStagedChangesReportFileAndLine(t *testing.T) {
	repo := newRepo(t)
	repo.Write("src/client.php", "<?php\n\n$token = "+token()+";\n").
		Write("untracked.php", "<?php\n$token = "+token()+";\n").
		Git("add", "src/client.php")

	findings := scan(t, repo, secrets.Target{Mode: secrets.StagedChanges})

	if got := locations(findings); !slices.Equal(got, []string{"src/client.php:3"}) {
		t.Fatalf("unexpected findings %v", got)
	}
	if findings[0].RuleID != "github-pat" || findings[0].Commit != "" {
		t.Errorf("unexpected finding %+v", findings[0])
	}
}

func TestTrackedFilesSkipUntrackedAndIgnoredFiles(t *testing.T) {
	repo := newRepo(t)
	repo.Write(".gitignore", ".env.local\nvar/\n").
		Write("config/token.php", "<?php return "+token()+";\n").
		Commit("Leak")
	repo.Write(".env.local", "GITHUB_TOKEN="+testutil.FakeGitHubToken()+"\n").
		Write("var/cache/token.php", "<?php return "+token()+";\n").
		Write("notes.txt", testutil.FakeGitHubToken()+"\n")

	if got := locations(scan(t, repo, secrets.Target{Mode: secrets.TrackedFiles})); !slices.Equal(got, []string{"config/token.php:1"}) {
		t.Errorf("unexpected findings %v", got)
	}
}

func TestCommitRangeIncludesSecretsRemovedLaterButNotEarlierCommits(t *testing.T) {
	repo := newRepo(t)
	base := repo.Write("old.php", "<?php return "+token()+";\n").Commit("Old leak")
	leak := repo.Write("new.php", "<?php return "+token()+";\n").Commit("New leak")
	repo.Delete("new.php").Commit("Remove leak")

	findings := scan(t, repo, secrets.Target{Mode: secrets.CommitRange, Base: base})

	if got := locations(findings); !slices.Equal(got, []string{"new.php:1"}) {
		t.Fatalf("unexpected findings %v", got)
	}
	if findings[0].Commit != leak || findings[0].Author != "Test User" || findings[0].Fingerprint != leak+":new.php:github-pat:1" {
		t.Errorf("unexpected finding %+v", findings[0])
	}
}

func TestHistoryIncludesOtherBranches(t *testing.T) {
	repo := newRepo(t)
	repo.Git("checkout", "--quiet", "-b", "feature")
	repo.Write("feature.php", "<?php return "+token()+";\n").Commit("Leak")
	repo.Git("checkout", "--quiet", "master")

	if got := locations(scan(t, repo, secrets.Target{Mode: secrets.History})); !slices.Equal(got, []string{"feature.php:1"}) {
		t.Errorf("unexpected findings %v", got)
	}
}

func TestIgnoreFileAndInlineAllowCommentSuppressFindings(t *testing.T) {
	repo := newRepo(t)
	commit := repo.Write("ignored.php", "<?php return "+token()+";\n").
		Write("allowed.php", "<?php return "+token()+"; // betterleaks:allow\n").
		Write("reported.php", "<?php return "+token()+";\n").
		Commit("Leaks")
	repo.Write(secrets.IgnoreFile, "# accepted\n"+commit+":ignored.php:github-pat:1\n")

	if got := locations(scan(t, repo, secrets.Target{Mode: secrets.History})); !slices.Equal(got, []string{"reported.php:1"}) {
		t.Errorf("unexpected findings %v", got)
	}
}

func TestProjectConfigExtendsNetgenRuleset(t *testing.T) {
	repo := newRepo(t)
	fakeToken := token()
	repo.Write(secrets.ProjectConfigFile, `
prefilter = '''
matchesAny(attributes["path"], [`+"`^tests/fixtures/`"+`])
'''

[[rules]]
id = 'acme-api-key'
description = 'ACME API key'
regex = '''\bacme_[a-z0-9]{24}\b'''
keywords = ['acme_']

[extend]
disabledRules = ['private-key-file']
`).
		Write("tests/fixtures/token.php", "<?php return "+fakeToken+";\n").
		Write("node_modules/lib/token.js", "module.exports = "+fakeToken+";\n").
		Write("src/token.php", "<?php return "+fakeToken+";\n").
		Write("src/acme.php", "<?php return 'acme_k2j9d8f7g6h5j4k3l2m1n0b9';\n").
		Write(".ssh/id_rsa", "not really a key\n").
		Write(".env", "APP_SECRET=7f3d9a1c2b4e6f8a0d1c3e5f7a9b1d2e\n").
		Commit("Project config")

	var results []string
	for _, finding := range scan(t, repo, secrets.Target{Mode: secrets.TrackedFiles}) {
		results = append(results, finding.RuleID+" "+finding.Location())
	}
	slices.Sort(results)

	// tests/ excluded by the project, node_modules/ by Betterleaks, private-key-file disabled
	expected := []string{"acme-api-key src/acme.php:1", "dotenv-secret .env:1", "github-pat src/token.php:1"}
	if !slices.Equal(results, expected) {
		t.Errorf("expected %v, got %v", expected, results)
	}
}

func TestInvalidProjectConfig(t *testing.T) {
	tests := map[string]string{
		"own extend path": "[extend]\npath = 'other.toml'\n",
		"useDefault":      "[extend]\nuseDefault = false\n",
		"filter type":     "filter = 42\n",
		"invalid TOML":    "prefilter = '''\ntrue\n",
	}

	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newRepo(t)
			repo.Write(secrets.ProjectConfigFile, config)

			_, err := secrets.Scanner{Binary: testutil.Betterleaks(t), Ruleset: ruleset.TOML}.Scan(repo.Root, secrets.Target{Mode: secrets.TrackedFiles})
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
