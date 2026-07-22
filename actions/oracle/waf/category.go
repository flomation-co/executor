package waf

// Sub-category metadata for the Web Application Firewall provider under Oracle Cloud. The middle
// path segment "waf" nests every oracle/waf/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are for
// manifest completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata
// entry or the palette header drifts.
const (
	CategoryName        = "Web Application Firewall"
	CategoryIcon        = "shield-virus"
	CategoryDescription = "Oracle Cloud Web Application Firewall — protect applications with web-app firewalls and reusable policies, manage network address lists, and browse protection capabilities"
)
