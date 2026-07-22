package dns

// Sub-category metadata for the DNS provider under Oracle Cloud. The middle path
// segment "dns" nests every oracle/dns/<verb> action under this sub-group. The api
// recomputes display metadata from its own in-code maps at serve time
// (subCategoryMetadata), so these are for manifest completeness — the Description MUST
// stay byte-identical to the api's subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "DNS"
	CategoryIcon        = "globe"
	CategoryDescription = "Oracle Cloud DNS — manage public and private zones and their records, steer traffic with policies, and run private-DNS views, resolvers and TSIG keys"
)
