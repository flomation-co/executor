package monitoring

// Sub-category metadata for the Monitoring provider under Oracle Cloud. The middle path segment
// "monitoring" nests every oracle/monitoring/<verb> action under this sub-group. The api
// recomputes display metadata from its own in-code maps at serve time (subCategoryMetadata), so
// these are for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Monitoring"
	CategoryIcon        = "gauge"
	CategoryDescription = "Oracle Cloud Monitoring — query metrics with the Monitoring Query Language, publish custom metrics, and create alarms that watch a metric and fire to a destination, with status, history and suppressions"
)
