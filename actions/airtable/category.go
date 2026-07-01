package airtable_common

// Category metadata for the Airtable provider. The manifest generator applies
// these to every airtable/* action as the top-level category. Airtable is a
// standalone category (like Notion and Webflow); the CRM/Infrastructure-style
// parent grouping is reserved for domains with multiple providers.
const (
	CategoryName        = "Airtable"
	CategoryIcon        = "airtable"
	CategoryDescription = "Manage bases, tables, and records in Airtable"
)
