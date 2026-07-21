package networking

// Sub-category metadata for the Networking provider under Oracle Cloud. The middle
// path segment "networking" nests every oracle/networking/<verb> action under this
// sub-group. The api recomputes display metadata from its own in-code maps at serve
// time (subCategoryMetadata), so these are for manifest completeness — the
// Description MUST stay byte-identical to the api's subCategoryMetadata entry or the
// palette group header drifts.
const (
	CategoryName        = "Networking"
	CategoryIcon        = "network-wired"
	CategoryDescription = "Oracle Cloud Networking — VCNs, subnets, security lists, route tables, gateways, network security groups, DHCP options and public IPs"
)
