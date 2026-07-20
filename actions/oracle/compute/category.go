package compute

// Sub-category metadata for the Compute (Instances) provider under Oracle Cloud.
// Shared with common.go (same package). The middle path segment "compute" nests
// every oracle/compute/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (see
// subCategoryMetadata), so these are for manifest completeness.
const (
	CategoryName        = "Compute"
	CategoryIcon        = "server"
	CategoryDescription = "Oracle Cloud Compute — instance lifecycle, shapes, images, VNICs, networking and tags"
)
