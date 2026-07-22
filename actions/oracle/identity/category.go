package identity

// Sub-category metadata for the Identity (IAM) provider under Oracle Cloud. The middle
// path segment "identity" nests every oracle/identity/<verb> action under this
// sub-group. The api recomputes display metadata from its own in-code maps at serve
// time (subCategoryMetadata), so these are for manifest completeness — the Description
// MUST stay byte-identical to the api's subCategoryMetadata entry or the palette header
// drifts.
const (
	CategoryName        = "Identity"
	CategoryIcon        = "shield-halved"
	CategoryDescription = "Oracle Cloud Identity (IAM) — users, groups and memberships, policies, compartments, dynamic groups, credentials, tagging, federation and identity domains"
)
