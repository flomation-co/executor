// Package marketing declares the top-level "Marketing" action category.
//
// The consts below are harvested by cmd/manifest/manifest.go into
// manifest.json; the api's categoryMetadata map (api/internal/http/action.go)
// is what the editor reads at serve time, so keep the two in sync. Marketing
// uses 3-segment action IDs (marketing/sendgrid/mail_send), so the sub-group
// (SendGrid) is resolved from the category.go in each provider directory and,
// on the api side, from subCategoryMetadata.
package marketing

const (
	CategoryName        = "Marketing"
	CategoryIcon        = "bullhorn"
	CategoryDescription = "Email and marketing platforms — contacts, campaigns, and transactional email"
)
