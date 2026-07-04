// Package devops declares the top-level "DevOps" action category.
//
// It is the home for continuous-integration / continuous-delivery platforms —
// Jenkins today, with room for GitHub Actions, GitLab CI, CircleCI and the like
// as sibling sub-groups later. The consts below are harvested by
// cmd/manifest/manifest.go into manifest.json; the api's categoryMetadata map
// (api/internal/http/action.go) is what the editor reads at serve time, so keep
// the two in sync.
package devops

const (
	CategoryName        = "DevOps"
	CategoryIcon        = "gears"
	CategoryDescription = "Automate your build, test, and deploy workflows — trigger jobs, watch builds, and manage your CI/CD servers"
)
