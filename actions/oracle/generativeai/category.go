package generativeai

// Sub-category metadata for the Generative AI provider under Oracle Cloud. The middle path segment
// "generativeai" nests every oracle/generativeai/<verb> action under this sub-group. The api
// recomputes display metadata from its own in-code maps at serve time (subCategoryMetadata), so
// these are for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Generative AI"
	CategoryIcon        = "robot"
	CategoryDescription = "Oracle Cloud Generative AI — run chat, text generation, summarization, embeddings and reranking against pretrained and custom large language models, and manage endpoints and dedicated AI clusters"
)
