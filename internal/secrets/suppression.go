package secrets

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// IgnoreFile is the repository's list of accepted findings, one fingerprint per line.
const IgnoreFile = ".betterleaksignore"

// Suppression is a change that hides findings from the scan: an inline allow comment,
// an ignore file entry or a project config change. Reported for pull requests so
// reviewers can check them.
type Suppression struct {
	File    string
	Line    int
	Reason  string
	Content string
}

// Location returns "file:line".
func (s Suppression) Location() string {
	return s.File + ":" + strconv.Itoa(s.Line)
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// SuppressionsFromDiff finds the suppressions added in a `git diff --unified=0`.
func SuppressionsFromDiff(diff string) []Suppression {
	var suppressions []Suppression
	file := ""
	line := 0
	configReported := map[string]bool{}

	for _, diffLine := range strings.Split(diff, "\n") {
		if strings.HasPrefix(diffLine, "+++ ") {
			file = strings.TrimPrefix(diffLine, "+++ b/")
			if file == diffLine {
				// "+++ /dev/null": the file was deleted
				file = ""
			}

			continue
		}

		if match := hunkHeader.FindStringSubmatch(diffLine); match != nil {
			line, _ = strconv.Atoi(match[1])

			continue
		}

		if file == "" || !strings.HasPrefix(diffLine, "+") {
			continue
		}

		content := strings.TrimSpace(diffLine[1:])
		lower := strings.ToLower(content)

		switch name := path.Base(file); {
		case name == IgnoreFile || name == ".gitleaksignore":
			if content != "" && !strings.HasPrefix(content, "#") {
				suppressions = append(suppressions, Suppression{file, line, "ignore file entry", content})
			}
		case name == ProjectConfigFile:
			if !configReported[file] {
				configReported[file] = true
				suppressions = append(suppressions, Suppression{file, line, "project config changed", content})
			}
		case strings.Contains(lower, "betterleaks:allow") || strings.Contains(lower, "gitleaks:allow"):
			suppressions = append(suppressions, Suppression{file, line, "inline allow comment", content})
		}

		line++
	}

	return suppressions
}
