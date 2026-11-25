package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/netgen/ngcheck/internal/betterleaks"
	"github.com/netgen/ngcheck/internal/ci"
	"github.com/netgen/ngcheck/internal/git"
	"github.com/netgen/ngcheck/internal/report"
	"github.com/netgen/ngcheck/internal/secrets"
	"github.com/netgen/ngcheck/ruleset"
)

const secretsUsage = `Usage: ngcheck secrets [options] [path]

Scans the git repository containing path (default: current directory) for leaked
secrets: passwords, API keys, tokens, private keys, credentials in DSNs...

Without a mode option it scans the commits of the pull request in GitHub Actions,
Bitbucket Pipelines and GitLab CI pull request jobs, and the current content of
all tracked files everywhere else.

The repository can extend the rules in .betterleaks.toml and accept findings in
.betterleaksignore. A "betterleaks:allow" comment ignores a single line.

Exit codes: 0 no secrets found, 1 secrets found, 2 error.

Options:
`

type secretsOptions struct {
	staged      bool
	since       string
	history     bool
	junit       string
	json        string
	writeIgnore bool
	verbose     bool
}

func runSecrets(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	var options secretsOptions

	flags := flag.NewFlagSet("secrets", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, secretsUsage)
		flags.PrintDefaults()
	}
	flags.BoolVar(&options.staged, "staged", false, "scan the changes staged for commit (pre-commit hook)")
	flags.StringVar(&options.since, "since", "", "scan the commits added after `ref` (ref..HEAD)")
	flags.BoolVar(&options.history, "history", false, "scan every commit of every branch")
	flags.StringVar(&options.junit, "junit", "", "also write a JUnit XML report to `file`")
	flags.StringVar(&options.json, "json", "", "also write a JSON report to `file`")
	flags.BoolVar(&options.writeIgnore, "write-ignore", false, "accept the current findings by adding them to "+secrets.IgnoreFile)
	flags.BoolVar(&options.verbose, "verbose", false, "show finding fingerprints")

	positional, err := parseInterleaved(flags, args)
	if errors.Is(err, flag.ErrHelp) {
		return ExitOK
	}
	if err != nil {
		return ExitError
	}
	if len(positional) > 1 {
		return fail(stderr, errors.New("only one path can be scanned"))
	}

	path := "."
	if len(positional) == 1 {
		path = positional[0]
	}

	context := ci.Detect(getenv)

	repository, err := git.Open(path)
	if err != nil {
		return fail(stderr, err)
	}

	target, mergeBase, err := resolveTarget(options, repository, context)
	if err != nil {
		return fail(stderr, err)
	}

	binary, err := betterleaks.Locate(getenv)
	if err != nil {
		return fail(stderr, err)
	}

	suffix := ""
	if context.Platform != "" {
		suffix = " (" + string(context.Platform) + ")"
	}
	fmt.Fprintf(stdout, "Scanning %s in %s%s\n", target.Describe(), repository.Root, suffix)

	findings, err := secrets.Scanner{Binary: binary, Ruleset: ruleset.TOML}.Scan(repository.Root, target)
	if err != nil {
		return fail(stderr, err)
	}

	var suppressions []secrets.Suppression
	if mergeBase != "" {
		diff, err := repository.Diff(mergeBase, "HEAD")
		if err != nil {
			return fail(stderr, err)
		}
		suppressions = secrets.SuppressionsFromDiff(diff)
	}

	if options.junit != "" {
		if err := report.WriteJUnit(findings, options.junit); err != nil {
			return fail(stderr, err)
		}
	}
	if options.json != "" {
		if err := report.WriteJSON(findings, options.json); err != nil {
			return fail(stderr, err)
		}
	}

	printSuppressions(stdout, context, suppressions)

	if len(findings) == 0 {
		fmt.Fprintln(stdout, "\nOK: no secrets found.")
		return ExitOK
	}

	printFindings(stdout, context, target, findings, options.verbose)

	if options.writeIgnore {
		added, err := secrets.AcceptFindings(repository.Root, findings, "Accepted with ngcheck secrets --write-ignore on "+time.Now().Format(time.DateOnly))
		if err != nil {
			return fail(stderr, err)
		}

		fmt.Fprintf(stdout, "\nAdded %d fingerprint(s) to %s. Review and commit it.\n", added, secrets.IgnoreFile)
		return ExitOK
	}

	accept := `, or accept the finding with "ngcheck secrets --write-ignore"`
	if target.Mode == secrets.StagedChanges {
		accept = ""
	}

	fmt.Fprintf(stdout, `
ERROR: %d potential secret(s) found.

 * A real secret: remove it and ROTATE it. A secret that was ever pushed is leaked,
   rewriting git history does not undo that.
 * A false positive: add a "betterleaks:allow" comment on that line or exclude the
   path in .betterleaks.toml%s.
`, len(findings), accept)

	return ExitFindings
}

// resolveTarget returns what to scan and, for pull requests, the merge base used to report suppressions.
func resolveTarget(options secretsOptions, repository *git.Repository, context ci.Context) (secrets.Target, string, error) {
	modes := 0
	for _, set := range []bool{options.staged, options.since != "", options.history} {
		if set {
			modes++
		}
	}

	switch {
	case modes > 1:
		return secrets.Target{}, "", errors.New("use only one of --staged, --since and --history")
	case options.staged && options.writeIgnore:
		return secrets.Target{}, "", errors.New(`--write-ignore cannot be used with --staged: fingerprints of uncommitted changes do not match once committed, use a "betterleaks:allow" comment instead`)
	case options.staged:
		return secrets.Target{Mode: secrets.StagedChanges}, "", nil
	case options.history:
		return secrets.Target{Mode: secrets.History}, "", nil
	case options.since != "":
		base, ok := repository.ResolveCommit(options.since)
		if !ok {
			return secrets.Target{}, "", fmt.Errorf("unknown ref %q", options.since)
		}

		return secrets.Target{Mode: secrets.CommitRange, Base: base}, "", nil
	case !context.IsPullRequest():
		return secrets.Target{Mode: secrets.TrackedFiles}, "", nil
	}

	base, ok := repository.ResolveCommit(context.BaseRef)
	if !ok && context.BaseBranch != "" && repository.FetchBranch(context.BaseBranch) {
		base, ok = repository.ResolveCommit(context.BaseRef)
	}
	if !ok {
		return secrets.Target{}, "", fmt.Errorf("cannot find the pull request base %q, use a full clone depth for this job", context.BaseRef)
	}

	mergeBase, ok := repository.MergeBase(base, "HEAD")
	if !ok && repository.IsShallow() && repository.Unshallow() {
		mergeBase, ok = repository.MergeBase(base, "HEAD")
	}
	if !ok {
		return secrets.Target{}, "", fmt.Errorf("no common ancestor of HEAD and the pull request base %q, use a full clone depth for this job", context.BaseRef)
	}

	return secrets.Target{Mode: secrets.CommitRange, Base: base}, mergeBase, nil
}

func printSuppressions(stdout io.Writer, context ci.Context, suppressions []secrets.Suppression) {
	if len(suppressions) == 0 {
		return
	}

	fmt.Fprintln(stdout, "\nWARNING: this pull request adds suppressions that hide findings from the scan. Reviewers should check each of them.")

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "LOCATION\tTYPE\tCHANGE")
	for _, s := range suppressions {
		fmt.Fprintf(table, "%s\t%s\t%s\n", s.Location(), s.Reason, s.Content)
	}
	table.Flush()

	if context.Platform == ci.GitHub {
		for _, s := range suppressions {
			fmt.Fprintln(stdout, report.GitHubAnnotation("warning", s.File, s.Line, "Secret scan suppression", s.Reason+": "+s.Content))
		}
	}
}

func printFindings(stdout io.Writer, context ci.Context, target secrets.Target, findings []secrets.Finding, verbose bool) {
	headers := []string{"RULE", "LOCATION"}
	if target.ScansCommits() {
		headers = append(headers, "COMMIT", "AUTHOR")
	}
	if verbose {
		headers = append(headers, "FINGERPRINT")
	}

	fmt.Fprintln(stdout)
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, strings.Join(headers, "\t"))

	for _, f := range findings {
		row := []string{f.RuleID, f.Location()}
		if target.ScansCommits() {
			author := f.Author
			if f.Email != "" {
				author += " <" + f.Email + ">"
			}
			row = append(row, fmt.Sprintf("%.8s", f.Commit), author)
		}
		if verbose {
			row = append(row, f.Fingerprint)
		}
		fmt.Fprintln(table, strings.Join(row, "\t"))
	}
	table.Flush()

	if context.Platform == ci.GitHub {
		for _, f := range findings {
			fmt.Fprintln(stdout, report.GitHubAnnotation("error", f.File, f.Line, "Potential secret: "+f.RuleID, f.Description))
		}
	}
}

// parseInterleaved parses flags that may come before or after positional arguments.
func parseInterleaved(flags *flag.FlagSet, args []string) ([]string, error) {
	var positional []string

	for {
		if err := flags.Parse(args); err != nil {
			return nil, err
		}

		args = flags.Args()
		if len(args) == 0 {
			return positional, nil
		}

		positional = append(positional, args[0])
		args = args[1:]
	}
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "ngcheck: error: %v\n", err)

	return ExitError
}
