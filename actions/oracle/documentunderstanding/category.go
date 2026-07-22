package documentunderstanding

// Sub-category metadata for the Document Understanding provider under Oracle Cloud. The middle path
// segment "documentunderstanding" nests every oracle/documentunderstanding/<verb> action under this
// sub-group. The api recomputes display metadata from its own in-code maps at serve time
// (subCategoryMetadata), so these are for manifest completeness — the Description MUST stay
// byte-identical to the api's subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Document Understanding"
	CategoryIcon        = "image"
	CategoryDescription = "Oracle Cloud Document Understanding — extract text, tables, key-values and classifications from documents, run processor jobs, and manage projects and models"
)
