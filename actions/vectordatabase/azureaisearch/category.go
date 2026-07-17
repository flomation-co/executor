package azureaisearch

// Sub-category metadata for the Azure AI Search provider under Vector
// Database. Shared with common.go (same package). The middle path segment
// "azureaisearch" makes every vectordatabase/azureaisearch/<verb> action nest
// under this sub-group. The api recomputes display metadata from its own
// in-code maps at serve time (see subCategoryMetadata), so these are for
// manifest completeness — the api copy must stay byte-identical.
const (
	CategoryName        = "Azure AI Search"
	CategoryIcon        = "magnifying-glass"
	CategoryDescription = "Azure AI Search — manage indexes and documents, and run keyword, vector, and hybrid queries"
)
