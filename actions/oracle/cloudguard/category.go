package cloudguard

// Sub-category metadata for the Cloud Guard provider under Oracle Cloud. The middle path segment
// "cloudguard" nests every oracle/cloudguard/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Cloud Guard"
	CategoryIcon        = "shield-halved"
	CategoryDescription = "Oracle Cloud Guard — monitor your tenancy's security posture with detector and responder recipes, targets and managed lists, and triage the problems Cloud Guard surfaces"
)
