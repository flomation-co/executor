// Sub-category metadata for Meta Ads under Marketing. The middle path segment
// "meta_ads" nests every marketing/meta_ads/<group>/<action> action under this
// sub-group, and each <group> directory adds a third level (Marketing > Meta
// Ads > Campaigns), the same shape as crm/apollo/<type>. Mirrored in the api's
// subCategoryMetadata / subSubCategoryMetadata maps at serve time.
package meta_ads_common

const (
	CategoryName        = "Meta Ads"
	CategoryIcon        = "facebook"
	CategoryDescription = "Create, adjust and report on Facebook and Instagram advertising via the Meta Marketing API"
)
