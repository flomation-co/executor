package datacatalog

// Sub-category metadata for the Data Catalog provider under Oracle Cloud. The middle path segment
// "datacatalog" nests every oracle/datacatalog/<verb> action under this sub-group. The api
// recomputes display metadata from its own in-code maps at serve time (subCategoryMetadata), so
// these are for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Data Catalog"
	CategoryIcon        = "book"
	CategoryDescription = "Oracle Cloud Data Catalog — organise data assets and connections, harvest entities, and build a business glossary of terms across your data estate"
)
