package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/netgen/ngchecks/internal/report"
	"github.com/netgen/ngchecks/internal/secrets"
)

func TestGitHubAnnotationEscapesValues(t *testing.T) {
	got := report.GitHubAnnotation("error", "dir,name:x%.php", 3, "Rule: x", "line one\n::warning::injected")
	expected := "::error file=dir%2Cname%3Ax%25.php,line=3,title=Rule%3A x::line one%0A::warning::injected"

	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestGitHubAnnotationOmitsLineForPathFindings(t *testing.T) {
	got := report.GitHubAnnotation("error", ".env.local", 0, "t", "m")

	if got != "::error file=.env.local,title=t::m" {
		t.Errorf("unexpected annotation %q", got)
	}
}

func TestWriteJUnit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reports", "secrets.xml")
	findings := []secrets.Finding{{RuleID: "github-pat", Description: "GitHub <token>", File: "a.php", Line: 2, Fingerprint: "a.php:github-pat:2"}}

	if err := report.WriteJUnit(findings, path); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		`<testsuite name="Secret leak protection" tests="1" failures="1">`,
		`<testcase classname="github-pat" name="a.php:2" file="a.php">`,
		`<failure message="GitHub &lt;token&gt;" type="github-pat">`,
	} {
		if !strings.Contains(string(content), expected) {
			t.Errorf("report does not contain %s:\n%s", expected, content)
		}
	}
}
