package apigateway

// Sub-category metadata for the API Gateway provider under Oracle Cloud. The middle path segment
// "apigateway" nests every oracle/apigateway/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "API Gateway"
	CategoryIcon        = "route"
	CategoryDescription = "Oracle Cloud API Gateway — publish and manage API gateways and their deployments, wire routes to backends, and version the API specifications they serve"
)
