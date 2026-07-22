package events

// Sub-category metadata for the Events provider under Oracle Cloud. The middle path segment
// "events" nests every oracle/events/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Events"
	CategoryIcon        = "bolt"
	CategoryDescription = "Oracle Cloud Events — create rules that match OCI event types (a resource created, updated or deleted) with a condition and fan them out to a stream, topic or function"
)
