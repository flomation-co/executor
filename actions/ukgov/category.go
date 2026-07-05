package ukgov_common

// Top-level "UK Government" category metadata. Each agency (Companies House,
// DVLA, Police, Food Standards, Environment Agency, postcode lookups) lives in
// a sub-directory with its own category.go, which the manifest generator emits
// as a sub-category so the editor renders "UK Government > Agency > Action".
const (
	CategoryName        = "UK Government"
	CategoryIcon        = "landmark"
	CategoryDescription = "UK government agency data — Companies House, DVLA, Police, Food Standards and more"
)
