package mysql

// Sub-category metadata for the MySQL HeatWave provider under Oracle Cloud. The middle path segment
// "mysql" nests every oracle/mysql/<verb> action under this sub-group. The api recomputes display
// metadata from its own in-code maps at serve time (subCategoryMetadata), so these are for manifest
// completeness — the Description MUST stay byte-identical to the api's subCategoryMetadata entry or
// the palette header drifts.
const (
	CategoryName        = "MySQL HeatWave"
	CategoryIcon        = "database"
	CategoryDescription = "Oracle Cloud MySQL HeatWave — provision and manage MySQL DB systems, take and manage backups, tune configurations, and run the in-memory HeatWave analytics cluster"
)
