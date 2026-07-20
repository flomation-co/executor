package objectstorage

// Sub-category metadata for the Object Storage provider under Oracle Cloud. The
// middle path segment "objectstorage" nests every oracle/objectstorage/<verb>
// action under this sub-group. The api recomputes display metadata from its own
// in-code maps at serve time (subCategoryMetadata), so these are for manifest
// completeness.
const (
	CategoryName        = "Object Storage"
	CategoryIcon        = "box"
	CategoryDescription = "Oracle Cloud Object Storage — buckets, objects, copy/rename, and pre-authenticated (presigned) request URLs"
)
