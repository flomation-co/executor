package messagebrokers

// Category metadata for the Message Brokers top-level group. The manifest
// generator harvests these consts as the category for every action under
// actions/messagebrokers/*; provider sub-groups (MQTT, ...) come from the
// category.go in each provider directory. The api recomputes display metadata
// from its own in-code maps at serve time, so these are for manifest
// completeness and parity with the other providers.
//
// The directory is "messagebrokers" (not "message_brokers") because the
// manifest generator derives Go package names and import aliases from the
// path; the display name lives in CategoryName.
const (
	CategoryName        = "Message Brokers"
	CategoryIcon        = "arrow-right-arrow-left"
	CategoryDescription = "Publish and subscribe to message brokers — move events between systems in real time"
)
