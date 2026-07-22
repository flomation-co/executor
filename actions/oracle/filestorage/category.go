package filestorage

// Sub-category metadata for the File Storage provider under Oracle Cloud. The middle
// path segment "filestorage" nests every oracle/filestorage/<verb> action under this
// sub-group. The api recomputes display metadata from its own in-code maps at serve time
// (subCategoryMetadata), so these are for manifest completeness — the Description MUST
// stay byte-identical to the api's subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "File Storage"
	CategoryIcon        = "folder-tree"
	CategoryDescription = "Oracle Cloud File Storage — provision NFS file systems and mount targets, wire exports, take and schedule snapshots, replicate across regions, and set quotas"
)
