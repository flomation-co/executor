package logging

// Sub-category metadata for the Logging provider under Oracle Cloud. The middle path segment
// "logging" nests every oracle/logging/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Logging"
	CategoryIcon        = "file-lines"
	CategoryDescription = "Oracle Cloud Logging — manage log groups and logs (service and custom), configure the unified monitoring agent, and search log content across your tenancy"
)
