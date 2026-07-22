package loadbalancer

// Sub-category metadata for the Load Balancer provider under Oracle Cloud. The
// middle path segment "loadbalancer" nests every oracle/loadbalancer/<verb> action
// under this sub-group. The api recomputes display metadata from its own in-code
// maps at serve time (subCategoryMetadata), so these are for manifest completeness —
// the Description MUST stay byte-identical to the api's subCategoryMetadata entry or
// the palette group header drifts.
const (
	CategoryName        = "Load Balancer"
	CategoryIcon        = "circle-nodes"
	CategoryDescription = "Oracle Cloud Load Balancer — provision Layer-7 load balancers, wire backend sets and listeners, manage SSL certificates, hostnames and routing, and track backend health"
)
