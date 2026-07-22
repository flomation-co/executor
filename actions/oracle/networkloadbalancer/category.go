package networkloadbalancer

// Sub-category metadata for the Network Load Balancer provider under Oracle Cloud.
// The middle path segment "networkloadbalancer" nests every
// oracle/networkloadbalancer/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so
// these are for manifest completeness — the Description MUST stay byte-identical to
// the api's subCategoryMetadata entry or the palette group header drifts.
const (
	CategoryName        = "Network Load Balancer"
	CategoryIcon        = "ethernet"
	CategoryDescription = "Oracle Cloud Network Load Balancer — provision Layer 3/4 (TCP/UDP) load balancers, wire backend sets and listeners, tune health checks, and track backend health"
)
