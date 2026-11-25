package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/netgen/ngcheck/internal/git"
	"github.com/netgen/ngcheck/internal/setup"
)

const initUsage = `Usage: ngcheck init [path]

Sets up the project containing path (default: current directory):
  - adds "ngcheck secrets --staged" to the pre-commit hook of CaptainHook
    (captainhook.json, captainhook.template.json) or Husky (.husky/pre-commit),
  - creates the GitHub Actions pull request check, or prints the Bitbucket
    Pipelines / GitLab CI step to add.

Running it again changes nothing.
`

func runInit(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, initUsage) }

	positional, err := parseInterleaved(flags, args)
	if errors.Is(err, flag.ErrHelp) {
		return ExitOK
	}
	if err != nil {
		return ExitError
	}
	if len(positional) > 1 {
		return fail(stderr, errors.New("only one path can be set up"))
	}

	path := "."
	if len(positional) == 1 {
		path = positional[0]
	}

	repository, err := git.Open(path)
	if err != nil {
		return fail(stderr, err)
	}

	steps, err := setup.Run(repository)
	if err != nil {
		return fail(stderr, err)
	}

	fmt.Fprintf(stdout, "Setting up ngcheck in %s\n\n", repository.Root)

	var changed []string

	for _, step := range steps {
		message := step.Message
		if step.File != "" {
			message = step.File + ": " + message
		}

		fmt.Fprintf(stdout, "%-8s %s\n", step.Status, message)

		if step.Snippet != "" {
			fmt.Fprintf(stdout, "\n%s\n\n", indent(step.Snippet, "    "))
		}

		if step.Status == setup.Changed {
			changed = append(changed, step.File)
		}
	}

	if len(changed) > 0 {
		fmt.Fprintf(stdout, "\nReview and commit the changes:\n\n    git add %s\n", strings.Join(changed, " "))
	}

	fmt.Fprintln(stdout, "\nTo find secrets committed before, run: ngcheck secrets --history")

	return ExitOK
}

func indent(text, prefix string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}

	return strings.Join(lines, "\n")
}
