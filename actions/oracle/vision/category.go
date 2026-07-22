package vision

// Sub-category metadata for the Vision provider under Oracle Cloud. The middle path segment
// "vision" nests every oracle/vision/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Vision"
	CategoryIcon        = "eye"
	CategoryDescription = "Oracle Cloud Vision — analyze images and documents for objects, text (OCR), classification and more, and manage vision projects and models"
)
