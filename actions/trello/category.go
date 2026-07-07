package trello_common

// Category metadata for the Trello top-level group. The manifest generator
// harvests these consts as the category for every action under actions/trello/*.
// Trello is grouped under the "Project Management" category alongside Jira; the
// api recomputes display metadata from its own in-code maps at serve time (see
// categoryMetadata / subCategoryMetadata), so these are for manifest
// completeness and parity with the other providers — the editor grouping is
// owned entirely by the api.
const (
	CategoryName        = "Trello"
	CategoryIcon        = "trello"
	CategoryDescription = "Create and manage boards, lists, cards, checklists, labels, and members in Trello"
)
