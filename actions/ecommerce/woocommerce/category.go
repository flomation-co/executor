package woocommerce

// Sub-category metadata for the WooCommerce provider under E-Commerce. Shared
// with common.go (same package). The middle path segment "woocommerce" makes
// every ecommerce/woocommerce/<verb> action nest under this sub-group, a sibling
// of ecommerce/shopify. The api recomputes display metadata from its own in-code
// maps at serve time (see subCategoryMetadata), so these are for manifest
// completeness and parity with the other providers.
const (
	CategoryName        = "WooCommerce"
	CategoryIcon        = "woocommerce"
	CategoryDescription = "Manage customers, orders, products, and coupons in your WooCommerce store"
)
