package compute

// Sub-category metadata for the Virtual Machines provider under Azure. Shared
// with common.go (same package). The middle path segment "compute" makes every
// azure/compute/<verb> action nest under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (see
// subCategoryMetadata), so these are for manifest completeness.
const (
	CategoryName        = "Virtual Machines"
	CategoryIcon        = "server"
	CategoryDescription = "Azure Virtual Machines — lifecycle, network security groups, disks, snapshots, images, SSH keys and tags"
)
