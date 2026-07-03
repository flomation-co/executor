package ecommerce

// Category metadata for the E-Commerce top-level group. The manifest
// generator harvests these consts as the category for every action under
// actions/ecommerce/*; provider sub-groups (Shopify, ...) come from the
// category.go in each provider directory. Note that the api recomputes
// display metadata from its own in-code maps at serve time, so these are
// for manifest completeness and parity with the other providers.
const (
	CategoryName        = "E-Commerce"
	CategoryIcon        = "cart-shopping"
	CategoryDescription = "Online store platforms — orders, products, and customers"
)
