package dataflow

// Sub-category metadata for the Data Flow provider under Oracle Cloud. The middle path segment
// "dataflow" nests every oracle/dataflow/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Data Flow"
	CategoryIcon        = "diagram-project"
	CategoryDescription = "Oracle Cloud Data Flow — a managed Apache Spark service: define applications, launch and manage runs, wire private endpoints, and submit interactive statements"
)
