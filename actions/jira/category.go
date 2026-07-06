package jira_common

// Category metadata for the Jira top-level group. The manifest generator harvests
// these consts as the category for every action under actions/jira/*. Jira is its
// own top-level category (like Linear and Notion), not a sub-group. The api
// recomputes display metadata from its own in-code maps at serve time (see
// categoryMetadata), so these are for manifest completeness and parity with the
// other providers.
const (
	CategoryName        = "Jira"
	CategoryIcon        = "jira"
	CategoryDescription = "Create and manage issues, comments, attachments, worklogs, and users in Jira"
)
