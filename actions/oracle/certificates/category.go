package certificates

// Sub-category metadata for the Certificates provider under Oracle Cloud. The middle path segment
// "certificates" nests every oracle/certificates/<verb> action under this sub-group. The api
// recomputes display metadata from its own in-code maps at serve time (subCategoryMetadata), so
// these are for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Certificates"
	CategoryIcon        = "id-badge"
	CategoryDescription = "Oracle Cloud Certificates — issue and manage TLS certificates and certificate authorities, bundle CA chains, rotate and revoke versions, and read the issued bundles"
)
