// Package report writes scan results in machine readable formats.
package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/netgen/ngcheck/internal/secrets"
)

type jsonFinding struct {
	Rule        string `json:"rule"`
	Description string `json:"description"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Fingerprint string `json:"fingerprint"`
	Commit      string `json:"commit,omitempty"`
	Author      string `json:"author,omitempty"`
	Email       string `json:"email,omitempty"`
	Date        string `json:"date,omitempty"`
}

// WriteJSON writes the findings as a JSON array.
func WriteJSON(findings []secrets.Finding, path string) error {
	entries := make([]jsonFinding, 0, len(findings))
	for _, f := range findings {
		entries = append(entries, jsonFinding{f.RuleID, f.Description, f.File, f.Line, f.Fingerprint, f.Commit, f.Author, f.Email, f.Date})
	}

	content, err := json.MarshalIndent(entries, "", "    ")
	if err != nil {
		return err
	}

	return writeFile(path, append(content, '\n'))
}

type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	ClassName string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	File      string        `xml:"file,attr,omitempty"`
	Failure   *junitFailure `xml:"failure"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// WriteJUnit writes a JUnit XML report with one failed test case per finding, or a
// single passing one. Bitbucket Pipelines picks it up from test-reports/, GitLab CI
// through artifacts:reports:junit.
func WriteJUnit(findings []secrets.Finding, path string) error {
	suite := junitSuite{Name: "Secret leak protection", Tests: len(findings), Failures: len(findings)}

	if len(findings) == 0 {
		suite.Tests = 1
		suite.Cases = []junitCase{{ClassName: "ngcheck.secrets", Name: "No secrets found"}}
	}

	for _, f := range findings {
		suite.Cases = append(suite.Cases, junitCase{
			ClassName: f.RuleID,
			Name:      f.Location(),
			File:      f.File,
			Failure: &junitFailure{
				Message: f.Description,
				Type:    f.RuleID,
				Text: fmt.Sprintf("Rule: %s\nLocation: %s\nCommit: %s\nAuthor: %s\nFingerprint: %s",
					f.RuleID, f.Location(), orDash(f.Commit), orDash(f.Author), f.Fingerprint),
			},
		})
	}

	content, err := xml.MarshalIndent(suite, "", "    ")
	if err != nil {
		return err
	}

	return writeFile(path, append([]byte(xml.Header), append(content, '\n')...))
}

// GitHubAnnotation returns a GitHub Actions workflow command that annotates a file in the
// pull request. Values are escaped so file names or messages cannot inject other commands.
// See https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands
func GitHubAnnotation(level, file string, line int, title, message string) string {
	properties := []string{"file=" + escapeProperty(file)}
	if line > 0 {
		properties = append(properties, "line="+strconv.Itoa(line))
	}
	properties = append(properties, "title="+escapeProperty(title))

	return fmt.Sprintf("::%s %s::%s", level, strings.Join(properties, ","), escapeData(message))
}

var (
	dataEscaper     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscaper = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)

func escapeData(value string) string {
	return dataEscaper.Replace(value)
}

func escapeProperty(value string) string {
	return propertyEscaper.Replace(value)
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}

	return value
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, content, 0o644)
}
