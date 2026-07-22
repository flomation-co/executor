package queue

// Sub-category metadata for the Queue provider under Oracle Cloud. The middle path segment
// "queue" nests every oracle/queue/<verb> action under this sub-group. The api recomputes display
// metadata from its own in-code maps at serve time (subCategoryMetadata), so these are for
// manifest completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata
// entry or the palette header drifts.
const (
	CategoryName        = "Queue"
	CategoryIcon        = "list"
	CategoryDescription = "Oracle Cloud Queue — a lightweight managed message queue: create and manage queues, then put, get, delete and update messages with visibility timeouts, channels and dead-letter handling"
)
