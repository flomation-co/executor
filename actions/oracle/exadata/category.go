package exadata

// Sub-category metadata for the Exadata (Database Service on Dedicated Infrastructure)
// provider under Oracle Cloud. The middle path segment "exadata" nests every
// oracle/exadata/<verb> action under this sub-group. The api recomputes display metadata
// from its own in-code maps at serve time (subCategoryMetadata), so these are for manifest
// completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata
// entry or the palette header drifts.
const (
	CategoryName        = "Exadata"
	CategoryIcon        = "microchip"
	CategoryDescription = "Oracle Cloud Exadata Database Service on Dedicated Infrastructure — provision and manage cloud Exadata infrastructure and VM clusters, inspect DB servers and nodes, and schedule maintenance runs"
)
