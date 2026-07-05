package wordpress

// Sub-category metadata for the WordPress provider under CMS. Shared with
// common.go (same package). The middle path segment "wordpress" makes every
// cms/wordpress/<verb> action nest under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (see
// subCategoryMetadata), so these are for manifest completeness.
const (
	CategoryName        = "WordPress"
	CategoryIcon        = "wordpress"
	CategoryDescription = "Manage posts, pages, users, comments, categories, and tags on your WordPress site"
)
