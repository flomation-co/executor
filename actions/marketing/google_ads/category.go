// Sub-category metadata for Google Ads under Marketing. The middle path segment
// "google_ads" nests every marketing/google_ads/<group>/<action> action under
// this sub-group, and each <group> directory adds a third level (Marketing >
// Google Ads > Campaigns) — the same shape as marketing/meta_ads/<group>.
// Mirrored in the api's subCategoryMetadata / subSubCategoryMetadata maps at
// serve time.
package google_ads_common

const (
	CategoryName        = "Google Ads"
	CategoryIcon        = "googleads"
	CategoryDescription = "Build, adjust and report on Google advertising via the Google Ads API"
)
