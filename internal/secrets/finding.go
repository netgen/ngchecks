package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// Finding is a potential secret. The scanner always redacts secrets, so a
// finding never contains the secret value.
type Finding struct {
	RuleID      string `json:"RuleID"`
	Description string `json:"Description"`
	File        string `json:"File"`
	// Line is 0 for rules that match on the file path alone.
	Line        int    `json:"StartLine"`
	Fingerprint string `json:"Fingerprint"`
	Commit      string `json:"Commit"`
	Author      string `json:"Author"`
	Email       string `json:"Email"`
	Date        string `json:"Date"`
}

// Location returns "file:line", or "file" for path-only findings.
func (f Finding) Location() string {
	if f.Line > 0 {
		return f.File + ":" + strconv.Itoa(f.Line)
	}

	return f.File
}

func readReport(path string) ([]Finding, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the Betterleaks report: %w", err)
	}

	var findings []Finding
	if err := json.Unmarshal(content, &findings); err != nil {
		return nil, fmt.Errorf("cannot parse the Betterleaks report: %w", err)
	}

	return findings, nil
}
