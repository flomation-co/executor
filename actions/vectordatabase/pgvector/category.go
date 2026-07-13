package pgvector

// Sub-category metadata for the pgvector provider under Vector Database.
// Shared with common.go (same package). The middle path segment "pgvector"
// makes every vectordatabase/pgvector/<verb> action nest under this sub-group.
// The api recomputes display metadata from its own in-code maps at serve time
// (see subCategoryMetadata), so these are for manifest completeness.
//
// pgvector is an extension, not a product, so it has no logo of its own; the
// provider borrows the Font Awesome "database" glyph, which is what an operator
// looking for "my Postgres" will scan for.
const (
	CategoryName        = "pgvector"
	CategoryIcon        = "database"
	CategoryDescription = "Store and query embeddings in a PostgreSQL database with the pgvector extension"
)
