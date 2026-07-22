package language

// Sub-category metadata for the Language provider under Oracle Cloud. The middle path segment
// "language" nests every oracle/language/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Language"
	CategoryIcon        = "comments"
	CategoryDescription = "Oracle Cloud Language — detect language, sentiment, entities, key phrases and PII, classify and translate text, and manage custom language projects, models and endpoints"
)
