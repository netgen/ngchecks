// Package setup configures a project to run ngcheck: the pre-commit hook of its
// hook manager (CaptainHook or Husky) and the pull request check of its CI.
package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/netgen/ngcheck/internal/git"
)

const (
	// CaptainHookAction is the CaptainHook pre-commit action running ngcheck.
	CaptainHookAction = "vendor/bin/ngcheck secrets --staged"
	// HuskyCommand is the line running ngcheck in .husky/pre-commit.
	HuskyCommand = "ngcheck secrets --staged"

	// Any hook or pipeline containing this already runs ngcheck.
	marker = "ngcheck secrets"

	docsURL = "https://github.com/netgen/ngcheck/blob/master/docs/installation.md"
)

// Status of a setup step.
type Status string

const (
	Changed   Status = "changed"
	Unchanged Status = "ok"
	Todo      Status = "todo"
)

// Step is one result of the setup. Todo steps describe what has to be done by hand.
type Step struct {
	Status Status
	// File is the file the step is about, relative to the repository root, if any.
	File    string
	Message string
	// Snippet is a command or configuration to copy for Todo steps.
	Snippet string
}

type setup struct {
	repository *git.Repository
	steps      []Step
}

// Run sets up the repository and returns what it did and what is left to do.
// It is idempotent: a second run reports everything as already set up.
func Run(repository *git.Repository) ([]Step, error) {
	s := &setup{repository: repository}

	if err := s.hooks(); err != nil {
		return nil, err
	}

	if err := s.ci(); err != nil {
		return nil, err
	}

	return s.steps, nil
}

func (s *setup) hooks() error {
	captainHook := false
	edited := map[string]bool{}

	for _, file := range []string{"captainhook.json", "captainhook.template.json"} {
		if !s.exists(file) {
			continue
		}

		captainHook = true

		// netgen/media-site projects symlink captainhook.json to the committed template:
		// edit and report the file the link points to, once.
		target := s.resolve(file)
		if edited[target] {
			continue
		}
		edited[target] = true

		if err := s.addCaptainHookAction(target); err != nil {
			return err
		}
	}

	// CaptainHook is installed, but not configured yet.
	if !captainHook && s.exists("vendor/bin/captainhook") {
		captainHook = true
		if err := s.addCaptainHookAction("captainhook.json"); err != nil {
			return err
		}
	}

	switch {
	case s.exists("captainhook.json"):
		s.checkCaptainHookInstalled()
	case s.exists("captainhook.template.json"):
		s.add(Todo, "", "CaptainHook reads captainhook.json, which does not exist in this clone. Link it to the template and install the git hooks:",
			"ln -s captainhook.template.json captainhook.json\nvendor/bin/captainhook install --force")
	}

	husky := s.isDir(".husky")
	if husky {
		if err := s.addHuskyCommand(); err != nil {
			return err
		}
		s.checkHuskyActive()
	}

	if captainHook || husky {
		return nil
	}

	switch {
	case s.exists("composer.json"):
		s.add(Todo, "", "No git hook manager found. Install CaptainHook, then run ngcheck init again:",
			"composer require --dev captainhook/captainhook\nvendor/bin/ngcheck init")
	case s.exists("package.json"):
		s.add(Todo, "", "No git hook manager found. Set up Husky, then run ngcheck init again:",
			"npm install --save-dev husky\nnpx husky init\nnpm exec -- ngcheck init")
	default:
		s.add(Todo, "", "No git hook manager found. Run this command in the pre-commit git hook:", HuskyCommand)
	}

	return nil
}

func (s *setup) addCaptainHookAction(file string) error {
	config := newJSONObject()

	content, err := os.ReadFile(s.path(file))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err == nil {
		value, err := decodeJSON(content)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}

		var ok bool
		if config, ok = value.(*jsonObject); !ok {
			return fmt.Errorf("%s: expected a JSON object", file)
		}
	}

	hook := newJSONObject()
	if value, ok := config.get("pre-commit"); ok {
		if hook, ok = value.(*jsonObject); !ok {
			return fmt.Errorf(`%s: "pre-commit" must be an object`, file)
		}
	} else {
		config.set("pre-commit", hook)
	}

	actions := &jsonArray{}
	if value, ok := hook.get("actions"); ok {
		if actions, ok = value.(*jsonArray); !ok {
			return fmt.Errorf(`%s: "pre-commit.actions" must be an array`, file)
		}
	}

	configured := false
	for _, action := range actions.items {
		if object, ok := action.(*jsonObject); ok {
			if command, ok := object.values["action"].(string); ok && strings.Contains(command, marker) {
				configured = true
			}
		}
	}

	enabled := hook.values["enabled"] == true

	if configured && enabled {
		s.add(Unchanged, file, "the pre-commit hook already runs ngcheck", "")
		return nil
	}

	var changes []string

	if !enabled {
		hook.set("enabled", true)

		if len(actions.items) > 0 && !configured {
			changes = append(changes, fmt.Sprintf("enabled the pre-commit hook (it also runs its %d other action(s))", len(actions.items)))
		} else {
			changes = append(changes, "enabled the pre-commit hook")
		}
	}

	if !configured {
		action := newJSONObject()
		action.set("action", CaptainHookAction)
		actions.items = append([]any{action}, actions.items...)
		hook.set("actions", actions)
		changes = append(changes, fmt.Sprintf("added %q to the pre-commit hook", CaptainHookAction))
	}

	encoded, err := encodeJSON(config)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}

	if err := os.WriteFile(s.path(file), encoded, 0o644); err != nil {
		return err
	}

	s.add(Changed, file, strings.Join(changes, ", "), "")

	return nil
}

func (s *setup) checkCaptainHookInstalled() {
	hooksDir, err := s.repository.HooksDir()
	if err == nil {
		if content, err := os.ReadFile(filepath.Join(hooksDir, "pre-commit")); err == nil && strings.Contains(string(content), "captainhook") {
			return
		}
	}

	s.add(Todo, "", "The CaptainHook git hooks are not installed in this clone:", "vendor/bin/captainhook install --force")
}

func (s *setup) addHuskyCommand() error {
	file := ".husky/pre-commit"

	command := HuskyCommand
	if s.exists(".pnp.cjs") {
		// Yarn Plug'n'Play has no node_modules/.bin
		command = "yarn " + HuskyCommand
	}

	content, err := os.ReadFile(s.path(file))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if strings.Contains(string(content), marker) {
		s.add(Unchanged, file, "already runs ngcheck", "")
		return nil
	}

	// Run ngcheck first, before slower commands such as lint-staged or tests.
	updated := command + "\n" + string(content)
	if strings.HasPrefix(string(content), "#!") {
		shebang, rest, _ := strings.Cut(string(content), "\n")
		updated = shebang + "\n" + command + "\n" + rest
	}

	if err := os.WriteFile(s.path(file), []byte(updated), 0o755); err != nil {
		return err
	}

	s.add(Changed, file, fmt.Sprintf("added %q", command), "")

	return nil
}

func (s *setup) checkHuskyActive() {
	if strings.HasPrefix(s.repository.Config("core.hooksPath"), ".husky") {
		return
	}

	s.add(Todo, "", "Husky is not active in this clone (npm install activates it through the \"prepare\" script):", "npx husky")
}

func (s *setup) ci() error {
	remote := s.repository.RemoteURL("origin")

	github := s.isDir(".github/workflows") || strings.Contains(remote, "github.com")
	bitbucket := s.exists("bitbucket-pipelines.yml") || strings.Contains(remote, "bitbucket.org")
	gitlab := s.exists(".gitlab-ci.yml") || strings.Contains(remote, "gitlab")

	if github {
		if err := s.addGitHubWorkflow(); err != nil {
			return err
		}
	}

	if bitbucket {
		s.checkPipeline("bitbucket-pipelines.yml", bitbucketPipeline)
	}

	if gitlab {
		s.checkPipeline(".gitlab-ci.yml", gitlabPipeline)
	}

	if !github && !bitbucket && !gitlab {
		s.add(Todo, "", "No CI configuration found. Add the pull request check to the pipeline, see "+docsURL+"#ci-pipelines", "")
	}

	return nil
}

func (s *setup) addGitHubWorkflow() error {
	workflows, _ := filepath.Glob(s.path(".github/workflows/*.y*ml"))

	for _, workflow := range workflows {
		if content, err := os.ReadFile(workflow); err == nil && strings.Contains(string(content), marker) {
			relative, _ := filepath.Rel(s.repository.Root, workflow)
			s.add(Unchanged, filepath.ToSlash(relative), "already runs the pull request check", "")

			return nil
		}
	}

	file := ".github/workflows/secrets.yml"
	if s.exists(file) {
		file = ".github/workflows/ngcheck-secrets.yml"
	}

	if err := os.MkdirAll(s.path(".github/workflows"), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(s.path(file), []byte(gitHubWorkflow), 0o644); err != nil {
		return err
	}

	s.add(Changed, file, "created the pull request check", "")

	return nil
}

func (s *setup) checkPipeline(file, snippet string) {
	content, err := os.ReadFile(s.path(file))

	switch {
	case err != nil:
		s.add(Todo, file, "create it with the pull request check:", snippet)
	case strings.Contains(string(content), marker):
		s.add(Unchanged, file, "already runs the pull request check", "")
	default:
		s.add(Todo, file, "add the pull request check to it (ngcheck init does not edit existing pipelines):", snippet)
	}
}

func (s *setup) add(status Status, file, message, snippet string) {
	s.steps = append(s.steps, Step{Status: status, File: file, Message: message, Snippet: snippet})
}

// resolve returns the file a symlink points to, relative to the root, or the file itself.
func (s *setup) resolve(file string) string {
	target, err := filepath.EvalSymlinks(s.path(file))
	if err != nil {
		return file
	}

	root, err := filepath.EvalSymlinks(s.repository.Root)
	if err != nil {
		return file
	}

	relative, err := filepath.Rel(root, target)
	if err != nil || strings.HasPrefix(relative, "..") {
		return file
	}

	return filepath.ToSlash(relative)
}

func (s *setup) path(file string) string {
	return filepath.Join(s.repository.Root, filepath.FromSlash(file))
}

func (s *setup) exists(file string) bool {
	_, err := os.Stat(s.path(file))

	return err == nil
}

func (s *setup) isDir(file string) bool {
	info, err := os.Stat(s.path(file))

	return err == nil && info.IsDir()
}

const gitHubWorkflow = `name: Secret leak protection

on:
    pull_request: ~

jobs:
    secrets:
        name: Secret scan
        runs-on: ubuntu-latest
        container: ghcr.io/netgen/ngcheck:1
        steps:
            - uses: actions/checkout@v7
              with:
                  fetch-depth: 0
            - run: ngcheck secrets
`

const bitbucketPipeline = `definitions:
    steps:
        - step: &secrets
              name: Secret leak protection
              image: ghcr.io/netgen/ngcheck:1
              clone:
                  depth: full
              script:
                  - ngcheck secrets --junit test-reports/secrets.xml

pipelines:
    pull-requests:
        '**':
            - step: *secrets
`

const gitlabPipeline = `secrets:
    image: ghcr.io/netgen/ngcheck:1
    variables:
        GIT_DEPTH: 0
    rules:
        - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    script:
        - ngcheck secrets --junit secrets.xml
    artifacts:
        when: always
        reports:
            junit: secrets.xml
`
