// Package crm declares the top-level "CRM" action category.
//
// The consts below are harvested by cmd/manifest/manifest.go into
// manifest.json; the api's categoryMetadata map (api/internal/http/action.go)
// is what the editor reads at serve time, so keep the two in sync. CRM uses
// 3-segment action IDs (crm/salesforce/lead_create), so the sub-group
// (Salesforce) is resolved from the category.go in each provider directory
// and, on the api side, from subCategoryMetadata.
//
// HubSpot is the other member of this group but lives at actions/hubspot/
// with 2-segment IDs (hubspot/contact_create) that predate this directory.
// The api remaps that prefix onto the same "crm" Key, so both providers share
// one palette header without HubSpot's action IDs having to change — moving
// the directory would rename all 28 of its actions and break every saved flow
// that references one.
package crm

const (
	CategoryName        = "CRM"
	CategoryIcon        = "people-group"
	CategoryDescription = "Customer relationship management — contacts, companies, deals, and tickets"
)
