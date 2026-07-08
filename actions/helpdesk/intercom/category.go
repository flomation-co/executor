// Sub-category metadata for the Intercom provider under Helpdesk. Shared with
// common.go (same package). The middle path segment "intercom" makes every
// helpdesk/intercom/<verb> action nest under this sub-group. Mirrored in the
// api's subCategoryMetadata map at serve time.
package intercom

const (
	CategoryName        = "Intercom"
	CategoryIcon        = "intercom"
	CategoryDescription = "Manage contacts, companies, conversations, tickets, tags, notes, and articles in Intercom"
)
