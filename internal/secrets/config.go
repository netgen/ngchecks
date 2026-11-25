package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ProjectConfigFile is the repository's own Betterleaks config. It extends the Netgen ruleset.
const ProjectConfigFile = ".betterleaks.toml"

// Global expressions that Betterleaks replaces instead of merging when extending a config.
var combinedKeys = []string{"prefilter", "filter"}

// composeConfig writes the Betterleaks config for a repository into dir and returns its path.
//
// Without a project config, that is the Netgen ruleset. With one, a config is generated that
// extends the ruleset (which extends the Betterleaks defaults) and combines the project's global
// prefilter and filter with the ruleset's ones: Betterleaks itself would replace them, silently
// dropping all built-in exclusions such as node_modules/.
func composeConfig(repositoryRoot string, ruleset []byte, dir string) (string, error) {
	rulesetPath := filepath.Join(dir, "ruleset.toml")
	if err := os.WriteFile(rulesetPath, ruleset, 0o600); err != nil {
		return "", err
	}

	projectTOML, err := os.ReadFile(filepath.Join(repositoryRoot, ProjectConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return rulesetPath, nil
	}
	if err != nil {
		return "", err
	}

	project := map[string]any{}
	if _, err := toml.Decode(string(projectTOML), &project); err != nil {
		return "", fmt.Errorf("%s: %w", ProjectConfigFile, err)
	}

	base := map[string]any{}
	if _, err := toml.Decode(string(ruleset), &base); err != nil {
		return "", fmt.Errorf("ruleset: %w", err)
	}

	for _, key := range combinedKeys {
		value, ok := project[key]
		if !ok {
			continue
		}

		expression, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%s: %q must be a string", ProjectConfigFile, key)
		}

		if baseExpression, ok := base[key].(string); ok {
			project[key] = fmt.Sprintf("(\n%s\n)\n|| (\n%s\n)", strings.TrimSpace(baseExpression), strings.TrimSpace(expression))
		}
	}

	extend := map[string]any{}
	if value, ok := project["extend"]; ok {
		if extend, ok = value.(map[string]any); !ok {
			return "", fmt.Errorf("%s: extend must be a table", ProjectConfigFile)
		}

		for _, key := range []string{"path", "useDefault"} {
			if _, ok := extend[key]; ok {
				return "", fmt.Errorf("%s: [extend] must not set %q, the Netgen ruleset is extended automatically", ProjectConfigFile, key)
			}
		}
	}

	extend["path"] = rulesetPath
	project["extend"] = extend

	var buffer bytes.Buffer
	if err := toml.NewEncoder(&buffer).Encode(project); err != nil {
		return "", fmt.Errorf("%s: %w", ProjectConfigFile, err)
	}

	path := filepath.Join(dir, "config.toml")

	return path, os.WriteFile(path, buffer.Bytes(), 0o600)
}
