// Package helpdesk declares the top-level "Helpdesk" action category.
//
// The consts below are harvested by cmd/manifest/manifest.go into
// manifest.json; the api's categoryMetadata map (api/internal/http/action.go)
// is what the editor reads at serve time, so keep the two in sync. Helpdesk
// uses 3-segment action IDs (helpdesk/zendesk/ticket_create), so the sub-group
// (Zendesk) is resolved from the category.go in each provider directory and,
// on the api side, from subCategoryMetadata.
package helpdesk

const (
	CategoryName        = "Helpdesk"
	CategoryIcon        = "headset"
	CategoryDescription = "Customer support and ticketing platforms"
)
