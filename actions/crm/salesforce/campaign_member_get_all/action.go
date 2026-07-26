package crm_salesforce_campaign_member_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Campaign Members"
	Description  = "See who is signed up to a Salesforce campaign and where each of them has got to — invited, responded, registered, attended. Filter by status to pull just the people you need to chase."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-group"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// defaultMemberFields is the SELECT list used when the operator does not choose
// their own. salesforce.DefaultFields has no CampaignMember entry — it would
// fall back to Id,Name,LastModifiedDate — and the point of this list is who
// these people are and where they have got to, which needs the status and the
// lead/contact link as well as the name.
const defaultMemberFields = "Id,CampaignId,LeadId,ContactId,Name,Email,Type,Status,HasResponded,FirstRespondedDate,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "701... — the campaign whose members you want", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Leave blank for name, email, status and response, or list your own: Name, Email, Status, Title"},
	{Name: "campaign_member_status", Type: core.ConnectionTypeString, Label: "Member Status", Placeholder: "Only members at this status, e.g. Responded — must match your org's Campaign Member Status list for that campaign"},
	{Name: "responded_only", Type: core.ConnectionTypeBoolean, Label: "Responded Only", Placeholder: "Tick to list only the people who have responded"},
	{
		Name:        "member_type",
		Type:        core.ConnectionTypeString,
		Label:       "Leads Or Contacts",
		Placeholder: "Leave blank for both",
		Options: []core.ConnectionOption{
			{Name: "Leads Only", Value: "Lead"},
			{Name: "Contacts Only", Value: "Contact"},
		},
	},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"HasResponded","operator":"=","value":"false"},{"field":"City","operator":"=","value":"Leeds"}]`},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "LastName ASC, Status ASC"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default, up to 2000 — ignored when Return All is on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Tick to keep fetching pages until every member has been collected"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted", Placeholder: "Tick to include members sitting in the Salesforce Recycle Bin"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Campaign Members"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Records Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	campaignID := salesforce.OptionalString("campaign_id", inputs)
	if err := salesforce.ValidateRecordID(campaignID); err != nil {
		return nil, err
	}

	// The campaign condition comes first and everything is ANDed onto it. There
	// is deliberately no "match any" option here as there is on the campaign
	// list: an OR would let the campaign filter drop out of the query and return
	// every campaign member in the org, which is both wrong and expensive.
	conditions := []salesforce.Condition{{Field: "CampaignId", Operator: "=", Value: campaignID}}

	extra, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, extra...)

	if v := salesforce.OptionalString("campaign_member_status", inputs); v != "" {
		conditions = append(conditions, salesforce.Condition{Field: "Status", Operator: "=", Value: v})
	}
	if salesforce.OptionalBool("responded_only", inputs) {
		conditions = append(conditions, salesforce.Condition{Field: "HasResponded", Operator: "=", Value: "true"})
	}
	if v := salesforce.OptionalString("member_type", inputs); v != "" {
		// CampaignMember.Type is the read-only "Lead" or "Contact" marker
		// Salesforce stamps on each member, so this filters on the person's kind
		// rather than needing two separate null checks on LeadId/ContactId.
		conditions = append(conditions, salesforce.Condition{Field: "Type", Operator: "=", Value: v})
	}

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = defaultMemberFields
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// applyLimit is off for Return All: a LIMIT clause would cut the list short
	// at exactly the point the operator asked for everybody. Paging is then
	// bounded by Query's own MaxAllPages cap instead.
	//
	// BuildQueryTyped, not BuildQuery: the status, the member type and the JSON
	// filters all carry operator-supplied values, and whether Salesforce wants
	// one quoted depends on the field — HasResponded = false is valid where
	// HasResponded = 'false' is INVALID_FIELD. It resolves that from one cached
	// describe and falls back to the untyped rendering if describe is denied.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"CampaignMember",
		fields,
		conditions,
		false,
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, salesforce.OptionalBool("include_deleted", inputs))
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d member(s) of campaign %s across %d page(s); stopped at the %d-page safety cap — narrow the filters to see the rest", len(records), campaignID, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d member(s) of campaign %s across %d page(s)", len(records), campaignID, pages)
	default:
		// Deliberately not "x of y": under a LIMIT clause Salesforce reports
		// totalSize as the size of the limited batch, so quoting it as a total
		// would tell the operator there are no more when there may well be.
		out["tool_result"] = fmt.Sprintf("Found %d member(s) of campaign %s", len(records), campaignID)
	}
	return out, nil
}
