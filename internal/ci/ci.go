// Package ci detects the CI platform and pull request base from environment variables.
package ci

// Platform is a supported CI service.
type Platform string

const (
	GitHub    Platform = "GitHub Actions"
	Bitbucket Platform = "Bitbucket Pipelines"
	GitLab    Platform = "GitLab CI"
)

// Context describes the CI job ngcheck runs in. The zero value means "not in CI".
type Context struct {
	Platform Platform
	// BaseRef is the ref or commit the pull request is compared against.
	BaseRef string
	// BaseBranch is the target branch, fetched from origin when BaseRef is missing locally.
	BaseBranch string
}

// Detect reads the variables GitHub Actions, Bitbucket Pipelines and GitLab CI set for their jobs.
func Detect(getenv func(string) string) Context {
	switch {
	case getenv("GITHUB_ACTIONS") == "true":
		return remoteBranchContext(GitHub, getenv("GITHUB_BASE_REF"))
	case getenv("BITBUCKET_BUILD_NUMBER") != "":
		return remoteBranchContext(Bitbucket, getenv("BITBUCKET_PR_DESTINATION_BRANCH"))
	case getenv("GITLAB_CI") == "true":
		base := getenv("CI_MERGE_REQUEST_DIFF_BASE_SHA")
		if base == "" {
			return Context{Platform: GitLab}
		}

		return Context{Platform: GitLab, BaseRef: base, BaseBranch: getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")}
	default:
		return Context{}
	}
}

// IsPullRequest reports whether the job runs for a pull (merge) request.
func (c Context) IsPullRequest() bool {
	return c.BaseRef != ""
}

func remoteBranchContext(platform Platform, branch string) Context {
	if branch == "" {
		return Context{Platform: platform}
	}

	return Context{Platform: platform, BaseRef: "origin/" + branch, BaseBranch: branch}
}
