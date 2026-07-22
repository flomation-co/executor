package nosql

// Sub-category metadata for the NoSQL Database provider under Oracle Cloud. The middle path segment
// "nosql" nests every oracle/nosql/<verb> action under this sub-group. The api recomputes display
// metadata from its own in-code maps at serve time (subCategoryMetadata), so these are for manifest
// completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata entry or
// the palette header drifts.
const (
	CategoryName        = "NoSQL Database"
	CategoryIcon        = "table"
	CategoryDescription = "Oracle Cloud NoSQL Database — create and manage tables and indexes, read, update and delete rows, and run SQL queries against a fully managed NoSQL store"
)
