package azureservicebus

// Sub-category metadata for the Azure Service Bus provider under Message
// Brokers. Shared with common.go (same package). The middle path segment
// "azureservicebus" makes every messagebrokers/azureservicebus/<verb> action
// nest under this sub-group. The api recomputes display metadata from its own
// in-code maps at serve time (see subCategoryMetadata), so these are for
// manifest completeness.
//
// Service Bus sits here rather than under the Azure provider because the
// palette is two-tier and a capability category beats a vendor one: an
// operator wiring up messaging looks beside MQTT, not inside Azure. It wears
// the Azure mark so the vendor is still obvious at a glance.
const (
	CategoryName        = "Azure Service Bus"
	CategoryIcon        = "azure"
	CategoryDescription = "Azure Service Bus — send, receive and schedule messages on queues and topics, manage entities, and work the dead-letter queue"
)
