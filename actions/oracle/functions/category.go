package functions

// Sub-category metadata for the Functions provider under Oracle Cloud. The middle path segment
// "functions" nests every oracle/functions/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are for
// manifest completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata
// entry or the palette header drifts.
const (
	CategoryName        = "Functions"
	CategoryIcon        = "code"
	CategoryDescription = "Oracle Cloud Functions — a serverless platform: create and manage applications and their functions, browse pre-built functions, and invoke a function on demand with a payload"
)
