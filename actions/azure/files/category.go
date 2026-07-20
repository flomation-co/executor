package files

// Sub-category metadata for the Files provider under Azure. Shared with
// common.go (same package). The middle path segment "files" makes every
// azure/files/<verb> action nest under this sub-group, a sibling of
// azure/storage rather than a fold into it: Blob's sub-group is committed to
// "containers, blobs, tiers, tags" and a share is not a container — it is an
// SMB filesystem with real directories, quotas and permissions.
//
// The api recomputes display metadata from its own in-code maps at serve time
// (see subCategoryMetadata), so these are for manifest completeness — but they
// must stay byte-identical to the api's copy.
const (
	CategoryName        = "Files"
	CategoryIcon        = "folder-tree"
	CategoryDescription = "SMB file shares — shares, directories, files and shared access links"
)
