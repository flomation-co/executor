package bastion

// Sub-category metadata for the Bastion provider under Oracle Cloud. The middle path segment
// "bastion" nests every oracle/bastion/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Bastion"
	CategoryIcon        = "terminal"
	CategoryDescription = "Oracle Cloud Bastion — create bastions and managed SSH sessions for secure, time-limited access to private hosts without exposing them to the internet"
)
