package asana_common

// Category metadata for the Asana top-level group. The manifest generator
// harvests these consts as the category for every action under actions/asana/*.
// Asana is grouped under the "Project Management" category alongside Jira and
// Trello; the api recomputes display metadata from its own in-code maps at serve
// time (see categoryMetadata), so these are for manifest completeness — the
// editor grouping is owned entirely by the api.
const (
	CategoryName        = "Asana"
	CategoryIcon        = "asana"
	CategoryDescription = "Create and manage tasks, subtasks, projects, sections, tags, and users in Asana"
)
