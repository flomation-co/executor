// Category metadata for the DevOps ▸ Azure DevOps sub-group. Harvested by
// cmd/manifest/manifest.go; mirrored in the api's subCategoryMetadata map
// (api/internal/http/action.go), so keep the two byte-identical.
//
// This node lives under DevOps beside Jenkins rather than under Azure beside
// Storage/Cosmos/Entra: it is a CI/CD product that happens to be Microsoft-
// branded, and operators reach for it by the job to be done. devops/category.go
// already anticipates siblings; the placement is hard to change later, since
// moving a node between categories changes every action ID.
package azuredevops

const (
	CategoryName        = "Azure DevOps"
	CategoryIcon        = "azure"
	CategoryDescription = "Azure DevOps — run pipelines, watch builds, manage work items, and review pull requests"
)
