package mqtt

// Sub-category metadata for the MQTT provider under Message Brokers. Shared
// with common.go (same package). The middle path segment "mqtt" makes every
// messagebrokers/mqtt/<verb> action nest under this sub-group. The api
// recomputes display metadata from its own in-code maps at serve time (see
// subCategoryMetadata), so these are for manifest completeness.
//
// MQTT's own logo is a wordmark that is illegible at node/palette size, so the
// provider uses the Font Awesome 6 "tower-broadcast" glyph, which reads as
// publish/subscribe at a glance.
const (
	CategoryName        = "MQTT"
	CategoryIcon        = "tower-broadcast"
	CategoryDescription = "Publish messages to an MQTT broker, read retained values, and wait for messages on a topic"
)
