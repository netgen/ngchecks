package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/netgen/ngcheck/internal/git"
	"github.com/netgen/ngcheck/internal/setup"
	"github.com/netgen/ngcheck/internal/testutil"
)

func run(t *testing.T, repo *testutil.Repo) []setup.Step {
	t.Helper()

	steps, err := setup.Run(&git.Repository{Root: repo.Root})
	if err != nil {
		t.Fatal(err)
	}

	return steps
}

func read(t *testing.T, repo *testutil.Repo, file string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repo.Root, file))
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func find(steps []setup.Step, file string) (setup.Step, bool) {
	for _, step := range steps {
		if step.File == file {
			return step, true
		}
	}

	return setup.Step{}, false
}

func hasTodo(steps []setup.Step, text string) bool {
	for _, step := range steps {
		if step.Status == setup.Todo && strings.Contains(step.Message+step.Snippet, text) {
			return true
		}
	}

	return false
}

func assertStatus(t *testing.T, steps []setup.Step, file string, status setup.Status) {
	t.Helper()

	step, ok := find(steps, file)
	if !ok || step.Status != status {
		t.Errorf("expected %s to be %q, got %+v", file, status, steps)
	}
}

const captainHookConfig = `{
    "commit-msg": {
        "enabled": true,
        "actions": [
            {
                "action": "\\Netgen\\GitHooks\\Action\\Regex",
                "options": {
                    "regex": "/^[A-Z0-9]+-\\d+:?\\s/",
                    "error": "Your commit message MUST start with a ticket ID"
                }
            }
        ]
    },
    "pre-commit": {
        "enabled": true,
        "actions": [
            {
                "action": "\\Netgen\\GitHooks\\Action\\PHPStan",
                "options": {
                    "excluded_files": ["/^deploy.*/"],
                    "level": 8
                }
            }
        ]
    },
    "config": {
        "php-path": "/usr/bin/env php8.4",
        "verbosity": null,
        "ansi-colors": false
    }
}
`

func TestAddsActionToCaptainHookConfigsKeepingTheirContent(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("captainhook.json", captainHookConfig).
		Write("captainhook.template.json", `{"pre-commit": {"enabled": false, "actions": []}}`)

	steps := run(t, repo)

	assertStatus(t, steps, "captainhook.json", setup.Changed)
	assertStatus(t, steps, "captainhook.template.json", setup.Changed)

	expected := strings.Replace(captainHookConfig, `    "pre-commit": {
        "enabled": true,
        "actions": [
`, `    "pre-commit": {
        "enabled": true,
        "actions": [
            {
                "action": "vendor/bin/ngcheck secrets --staged"
            },
`, 1)
	expected = strings.Replace(expected, `"excluded_files": ["/^deploy.*/"],`, `"excluded_files": [
                        "/^deploy.*/"
                    ],`, 1)

	if got := read(t, repo, "captainhook.json"); got != expected {
		t.Errorf("unexpected captainhook.json:\n%s", got)
	}

	expectedTemplate := `{
    "pre-commit": {
        "enabled": true,
        "actions": [
            {
                "action": "vendor/bin/ngcheck secrets --staged"
            }
        ]
    }
}
`
	if got := read(t, repo, "captainhook.template.json"); got != expectedTemplate {
		t.Errorf("unexpected captainhook.template.json:\n%s", got)
	}

	// Idempotent
	steps = run(t, repo)
	assertStatus(t, steps, "captainhook.json", setup.Unchanged)
	assertStatus(t, steps, "captainhook.template.json", setup.Unchanged)
}

func TestCreatesCaptainHookConfigWhenCaptainHookIsInstalled(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("composer.json", "{}\n").Write("vendor/bin/captainhook", "#!/usr/bin/env php\n")

	steps := run(t, repo)

	assertStatus(t, steps, "captainhook.json", setup.Changed)
	if !strings.Contains(read(t, repo, "captainhook.json"), `"action": "vendor/bin/ngcheck secrets --staged"`) {
		t.Errorf("action not added:\n%s", read(t, repo, "captainhook.json"))
	}
	if !hasTodo(steps, "vendor/bin/captainhook install") {
		t.Errorf("expected a todo to install the CaptainHook git hooks, got %+v", steps)
	}
}

func TestNoTodoWhenCaptainHookGitHooksAreInstalled(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write("captainhook.json", "{}\n").
		Write(".git/hooks/pre-commit", "#!/bin/sh\nvendor/bin/captainhook hook:pre-commit \"$@\"\n")

	if steps := run(t, repo); hasTodo(steps, "captainhook install") {
		t.Errorf("unexpected todo %+v", steps)
	}
}

func TestAddsCommandToHuskyHook(t *testing.T) {
	tests := map[string]struct {
		files    map[string]string
		expected string
	}{
		"before existing commands": {
			map[string]string{".husky/pre-commit": "npx lint-staged\n"},
			"ngcheck secrets --staged\nnpx lint-staged\n",
		},
		"after the shebang": {
			map[string]string{".husky/pre-commit": "#!/usr/bin/env sh\n. \"$(dirname -- \"$0\")/_/husky.sh\"\n\nnpm test\n"},
			"#!/usr/bin/env sh\nngcheck secrets --staged\n. \"$(dirname -- \"$0\")/_/husky.sh\"\n\nnpm test\n",
		},
		"new hook file": {
			map[string]string{".husky/commit-msg": "npx commitlint --edit $1\n"},
			"ngcheck secrets --staged\n",
		},
		"Yarn Plug'n'Play": {
			map[string]string{".husky/pre-commit": "yarn lint-staged\n", ".pnp.cjs": ""},
			"yarn ngcheck secrets --staged\nyarn lint-staged\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			repo := testutil.NewRepo(t)
			for file, content := range test.files {
				repo.Write(file, content)
			}

			assertStatus(t, run(t, repo), ".husky/pre-commit", setup.Changed)

			if got := read(t, repo, ".husky/pre-commit"); got != test.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", test.expected, got)
			}

			assertStatus(t, run(t, repo), ".husky/pre-commit", setup.Unchanged)
		})
	}
}

func TestReportsInactiveHusky(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write(".husky/pre-commit", "npm test\n")

	if steps := run(t, repo); !hasTodo(steps, "npx husky") {
		t.Errorf("expected a todo to activate Husky, got %+v", steps)
	}

	repo.Git("config", "core.hooksPath", ".husky/_")

	if steps := run(t, repo); hasTodo(steps, "npx husky") {
		t.Errorf("unexpected todo %+v", steps)
	}
}

func TestExplainsHowToInstallAHookManager(t *testing.T) {
	tests := map[string]struct{ file, todo string }{
		"PHP project":  {"composer.json", "composer require --dev captainhook/captainhook"},
		"Node project": {"package.json", "npx husky init"},
		"other":        {"README.md", "Run this command in the pre-commit git hook"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			repo := testutil.NewRepo(t)
			repo.Write(test.file, "{}\n")

			if steps := run(t, repo); !hasTodo(steps, test.todo) {
				t.Errorf("expected a todo containing %q, got %+v", test.todo, steps)
			}
		})
	}
}

func TestCreatesGitHubWorkflow(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Write(".github/workflows/ci.yml", "name: CI\n")

	assertStatus(t, run(t, repo), ".github/workflows/secrets.yml", setup.Changed)

	workflow := read(t, repo, ".github/workflows/secrets.yml")
	for _, expected := range []string{"pull_request", "container: ghcr.io/netgen/ngcheck:1", "fetch-depth: 0", "run: ngcheck secrets"} {
		if !strings.Contains(workflow, expected) {
			t.Errorf("workflow does not contain %q:\n%s", expected, workflow)
		}
	}

	assertStatus(t, run(t, repo), ".github/workflows/secrets.yml", setup.Unchanged)
}

func TestDetectsGitHubFromRemote(t *testing.T) {
	repo := testutil.NewRepo(t)
	repo.Git("remote", "add", "origin", "git@github.com:netgen/project.git")

	assertStatus(t, run(t, repo), ".github/workflows/secrets.yml", setup.Changed)
}

func TestPrintsPipelineSteps(t *testing.T) {
	t.Run("existing Bitbucket pipeline", func(t *testing.T) {
		repo := testutil.NewRepo(t)
		repo.Write("bitbucket-pipelines.yml", "pipelines:\n  default: []\n")

		steps := run(t, repo)

		assertStatus(t, steps, "bitbucket-pipelines.yml", setup.Todo)
		if step, _ := find(steps, "bitbucket-pipelines.yml"); !strings.Contains(step.Snippet, "ngcheck secrets --junit test-reports/secrets.xml") {
			t.Errorf("unexpected snippet %q", step.Snippet)
		}
		if read(t, repo, "bitbucket-pipelines.yml") != "pipelines:\n  default: []\n" {
			t.Error("bitbucket-pipelines.yml must not be changed")
		}
	})

	t.Run("Bitbucket remote without pipeline", func(t *testing.T) {
		repo := testutil.NewRepo(t)
		repo.Git("remote", "add", "origin", "git@bitbucket.org:netgen/project.git")

		if step, _ := find(run(t, repo), "bitbucket-pipelines.yml"); step.Status != setup.Todo || !strings.HasPrefix(step.Message, "create it") {
			t.Errorf("unexpected step %+v", step)
		}
	})

	t.Run("GitLab pipeline already running ngcheck", func(t *testing.T) {
		repo := testutil.NewRepo(t)
		repo.Write(".gitlab-ci.yml", "secrets:\n  script:\n    - ngcheck secrets\n")

		assertStatus(t, run(t, repo), ".gitlab-ci.yml", setup.Unchanged)
	})

	t.Run("no CI", func(t *testing.T) {
		repo := testutil.NewRepo(t)

		if steps := run(t, repo); !hasTodo(steps, "No CI configuration found") {
			t.Errorf("expected a todo, got %+v", steps)
		}
	})
}

func TestRejectsInvalidCaptainHookConfig(t *testing.T) {
	for name, config := range map[string]string{
		"invalid JSON":       "{",
		"not an object":      "[]",
		"pre-commit array":   `{"pre-commit": []}`,
		"actions not a list": `{"pre-commit": {"actions": {}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			repo := testutil.NewRepo(t)
			repo.Write("captainhook.json", config)

			if _, err := setup.Run(&git.Repository{Root: repo.Root}); err == nil {
				t.Error("expected an error")
			}
			if read(t, repo, "captainhook.json") != config {
				t.Error("captainhook.json must not be changed")
			}
		})
	}
}
