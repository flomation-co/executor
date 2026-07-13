package vectordatabase

// Category metadata for the Vector Database top-level group. The manifest
// generator harvests these consts as the category for every action under
// actions/vectordatabase/*; provider sub-groups (pgvector, ...) come from the
// category.go in each provider directory. The api recomputes display metadata
// from its own in-code maps at serve time, so these are for manifest
// completeness and parity with the other providers.
//
// The directory is "vectordatabase" (not "vector_database") because the
// manifest generator derives Go package names and import aliases from the
// path; the display name lives in CategoryName.
//
// A vector database stores text alongside the embedding that represents its
// meaning, so a flow can retrieve by *similarity* rather than by keyword. It is
// the storage half of retrieval-augmented generation: put your handbook in,
// then ask a question and get back the passages that actually answer it.
const (
	CategoryName        = "Vector Database"
	CategoryIcon        = "circle-nodes"
	CategoryDescription = "Store and search embeddings — semantic search, similarity lookups, and retrieval-augmented generation"
)
