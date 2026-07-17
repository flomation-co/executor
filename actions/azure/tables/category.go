package tables

// Sub-category metadata for the Table Storage provider under Azure. Shared
// with common.go (same package). The middle path segment "tables" makes every
// azure/tables/<verb> action nest under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (see
// subCategoryMetadata), so these are for manifest completeness.
//
// Table Storage is deliberately NOT folded into the Storage sub-group even
// though Azure bills them as one account with one set of access keys. That
// group's description is Blob-committed ("containers, blobs, tiers, tags, and
// shared access links") and an operator hunting "save a row to a table" would
// never scan it. Blobs are files; tables are rows — different mood, different
// hunt, and widening one description to cover both makes it vague enough to
// hurt the scanability of each half.
const (
	CategoryName        = "Table Storage"
	CategoryIcon        = "table"
	CategoryDescription = "Azure Table Storage — rows in a NoSQL key-value store: insert, query, update and batch entities"
)
