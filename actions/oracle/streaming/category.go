package streaming

// Sub-category metadata for the Streaming provider under Oracle Cloud. The middle path
// segment "streaming" nests every oracle/streaming/<verb> action under this sub-group. The
// api recomputes display metadata from its own in-code maps at serve time
// (subCategoryMetadata), so these are for manifest completeness — the Description MUST stay
// byte-identical to the api's subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Streaming"
	CategoryIcon        = "tower-broadcast"
	CategoryDescription = "Oracle Cloud Streaming — a Kafka-compatible managed streaming service: create and manage streams, stream pools and connect harnesses, publish messages, and consume them with cursors and consumer groups"
)
