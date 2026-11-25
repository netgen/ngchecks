package ci_test

import (
	"testing"

	"github.com/netgen/ngchecks/internal/ci"
)

func TestDetect(t *testing.T) {
	tests := map[string]struct {
		environment map[string]string
		expected    ci.Context
	}{
		"local": {nil, ci.Context{}},
		"GitHub pull request": {
			map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_BASE_REF": "master", "GITHUB_HEAD_REF": "feature"},
			ci.Context{Platform: ci.GitHub, BaseRef: "origin/master", BaseBranch: "master"},
		},
		"GitHub push": {
			map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_BASE_REF": ""},
			ci.Context{Platform: ci.GitHub},
		},
		"Bitbucket pull request": {
			map[string]string{"BITBUCKET_BUILD_NUMBER": "42", "BITBUCKET_PR_ID": "7", "BITBUCKET_PR_DESTINATION_BRANCH": "develop"},
			ci.Context{Platform: ci.Bitbucket, BaseRef: "origin/develop", BaseBranch: "develop"},
		},
		"Bitbucket branch pipeline": {
			map[string]string{"BITBUCKET_BUILD_NUMBER": "42", "BITBUCKET_BRANCH": "master"},
			ci.Context{Platform: ci.Bitbucket},
		},
		"GitLab merge request": {
			map[string]string{"GITLAB_CI": "true", "CI_MERGE_REQUEST_DIFF_BASE_SHA": "abc123", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main"},
			ci.Context{Platform: ci.GitLab, BaseRef: "abc123", BaseBranch: "main"},
		},
		"GitLab branch pipeline": {
			map[string]string{"GITLAB_CI": "true", "CI_COMMIT_BRANCH": "main"},
			ci.Context{Platform: ci.GitLab},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			getenv := func(name string) string { return test.environment[name] }

			if got := ci.Detect(getenv); got != test.expected {
				t.Errorf("expected %+v, got %+v", test.expected, got)
			}
		})
	}
}
