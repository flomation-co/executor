// Package scheduling declares the top-level "Scheduling" action category.
//
// The consts below are harvested by cmd/manifest/manifest.go into
// manifest.json; the api's categoryMetadata map (api/internal/http/action.go)
// is what the editor reads at serve time, so keep the two in sync.
package scheduling

const (
	CategoryName        = "Scheduling"
	CategoryIcon        = "calendar"
	CategoryDescription = "Meeting scheduling and booking platforms"
)
