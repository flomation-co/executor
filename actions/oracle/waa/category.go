package waa

// Sub-category metadata for the Web Application Acceleration provider under Oracle Cloud. The middle
// path segment "waa" nests every oracle/waa/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are for
// manifest completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata
// entry or the palette header drifts.
const (
	CategoryName        = "Web Application Acceleration"
	CategoryIcon        = "bolt"
	CategoryDescription = "Oracle Cloud Web Application Acceleration — speed up web apps with edge caching and compression policies on a load balancer, and purge the cache on demand"
)
