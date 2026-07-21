package autonomousdatabase

// Sub-category metadata for the Autonomous Database provider under Oracle Cloud.
// The middle path segment "autonomousdatabase" nests every
// oracle/autonomousdatabase/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata),
// so these are for manifest completeness — the Description MUST stay byte-identical
// to the api's subCategoryMetadata entry or the palette group header drifts.
const (
	CategoryName        = "Autonomous Database"
	CategoryIcon        = "database"
	CategoryDescription = "Oracle Cloud Autonomous Database — provision, scale, back up, clone and generate connection wallets for self-driving Oracle databases"
)
