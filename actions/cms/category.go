package cms

// Category metadata for the CMS top-level group. The manifest generator harvests
// these consts as the category for every action under actions/cms/*; provider
// sub-groups (WordPress, ...) come from the category.go in each provider
// directory. The api recomputes display metadata from its own in-code maps at
// serve time, so these are for manifest completeness and parity with the other
// providers.
const (
	CategoryName        = "CMS"
	CategoryIcon        = "newspaper"
	CategoryDescription = "Content management systems — publish and manage posts, pages, and media"
)
