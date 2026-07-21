package blockvolume

// Sub-category metadata for the Block Volumes provider under Oracle Cloud. The
// middle path segment "blockvolume" nests every oracle/blockvolume/<verb> action
// under this sub-group. The api recomputes display metadata from its own in-code
// maps at serve time (subCategoryMetadata), so these are for manifest completeness —
// the Description MUST stay byte-identical to the api's subCategoryMetadata entry or
// the palette group header drifts.
const (
	CategoryName        = "Block Volumes"
	CategoryIcon        = "hard-drive"
	CategoryDescription = "Oracle Cloud Block Volumes — provision and attach block/boot volumes, take and copy backups, and schedule them with backup policies"
)
