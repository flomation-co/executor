package monday_common

// Category metadata for the Monday.com top-level group. The manifest generator
// harvests these consts as the category for every action under actions/monday/*.
// Monday is grouped under the "Project Management" category alongside Jira,
// Trello and Asana; the api recomputes display metadata from its own in-code
// maps at serve time (see categoryMetadata), so these are for manifest
// completeness — the editor grouping is owned entirely by the api.
const (
	CategoryName        = "Monday.com"
	CategoryIcon        = "monday"
	CategoryDescription = "Create and manage boards, groups, columns, items, and updates in Monday.com"
)
