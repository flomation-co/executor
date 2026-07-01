package hubspot_common

// Category metadata for the HubSpot provider. The manifest generator
// applies these to every hubspot/* action as the top-level category;
// per-object sub-categories (Contacts, Companies, ...) come from the
// category.go in each object directory.
const (
	CategoryName        = "HubSpot"
	CategoryIcon        = "hubspot"
	CategoryDescription = "Manage contacts, companies, deals, and tickets in the HubSpot CRM"
)
