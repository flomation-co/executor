package email

// Sub-category metadata for the Email Delivery provider under Oracle Cloud. The middle path
// segment "email" nests every oracle/email/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are for
// manifest completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata
// entry or the palette header drifts.
const (
	CategoryName        = "Email"
	CategoryIcon        = "envelope"
	CategoryDescription = "Oracle Cloud Email Delivery — manage sending domains and DKIM signing, approved senders, and the suppression list for reliable transactional email"
)
