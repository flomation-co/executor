// Sub-category metadata for the Zendesk provider under Helpdesk. Shared with
// common.go (same package). The middle path segment "zendesk" makes every
// helpdesk/zendesk/<verb> action nest under this sub-group. Mirrored in the
// api's subCategoryMetadata map at serve time.
package zendesk

const (
	CategoryName        = "Zendesk"
	CategoryIcon        = "zendesk"
	CategoryDescription = "Manage tickets, users, and organizations in Zendesk Support"
)
