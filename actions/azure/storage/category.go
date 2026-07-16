package storage

// Sub-category metadata for the Storage provider under Azure. Shared with
// common.go (same package). The middle path segment "storage" makes every
// azure/storage/<verb> action nest under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (see
// subCategoryMetadata), so these are for manifest completeness.
const (
	CategoryName        = "Storage"
	CategoryIcon        = "box-archive"
	CategoryDescription = "Azure Blob Storage — containers, blobs, tiers, tags, and shared access links"
)
