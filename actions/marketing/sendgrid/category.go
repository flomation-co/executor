// Sub-category metadata for the SendGrid provider under Marketing. Shared with
// common.go (same package). The middle path segment "sendgrid" makes every
// marketing/sendgrid/<verb> action nest under this sub-group. Mirrored in the
// api's subCategoryMetadata map at serve time.
package sendgrid

const (
	CategoryName        = "SendGrid"
	CategoryIcon        = "sendgrid"
	CategoryDescription = "Send transactional email and manage contacts, lists, templates, and suppressions in SendGrid"
)
