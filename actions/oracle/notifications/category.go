package notifications

// Sub-category metadata for the Notifications (ONS) provider under Oracle Cloud. The middle
// path segment "notifications" nests every oracle/notifications/<verb> action under this
// sub-group. The api recomputes display metadata from its own in-code maps at serve time
// (subCategoryMetadata), so these are for manifest completeness — the Description MUST stay
// byte-identical to the api's subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Notifications"
	CategoryIcon        = "bell"
	CategoryDescription = "Oracle Cloud Notifications (ONS) — create topics, manage subscriptions across email, SMS, HTTPS, Slack and more, and publish a message that fans out to every subscriber"
)
