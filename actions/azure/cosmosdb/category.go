package cosmosdb

// Sub-category metadata for Cosmos DB under Azure. The middle path segment
// "cosmosdb" makes every azure/cosmosdb/<verb> action nest under this
// sub-group. The api recomputes display metadata from its own in-code maps at
// serve time (see subCategoryMetadata), so these are for manifest completeness
// — the Description must stay byte-identical to the api's copy.
const (
	CategoryName        = "Cosmos DB"
	CategoryIcon        = "database"
	CategoryDescription = "Azure Cosmos DB (NoSQL) — databases, containers, items, queries, and throughput"
)
