package crm_salesforce_campaign_member_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Remove Campaign Member"
	Description  = "Take someone off a Salesforce campaign when they cancel. Give the campaign member's ID, or just the campaign plus the lead or contact and they will be found for you. The lead or contact itself is untouched."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-minus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "campaign_member_id", Type: core.ConnectionTypeString, Label: "Campaign Member ID", Placeholder: "00v... — or leave blank and fill in the three boxes below instead"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "701... — used to find the member when you have not got their member ID"},
	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q... — the lead to remove from that campaign"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "003... — the contact to remove from that campaign"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign Member ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Removed Member"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	memberID, cfgErr, lookupErr := resolveMemberID(instanceURL, token, inputs)
	if cfgErr != nil {
		return nil, cfgErr
	}
	if lookupErr != nil {
		return salesforce.ErrorResult(lookupErr.Error()), nil
	}

	// Deleting the CampaignMember only breaks the link between the person and
	// the campaign — the lead or contact record itself is left alone, which is
	// exactly what "they cancelled" should mean.
	if err := salesforce.DeleteRecord(instanceURL, token, "CampaignMember", memberID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// A delete answers 204 No Content, so echo the ID back rather than returning
	// an empty result a later step cannot report on.
	removed := map[string]interface{}{"Id": memberID, "deleted": true}
	return salesforce.RecordResult(memberID, removed, fmt.Sprintf("Removed campaign member %s — the record is in the Recycle Bin and can be restored for 15 days", memberID)), nil
}

// resolveMemberID works out which CampaignMember record to remove.
//
// The operator can give its ID directly, but nobody taking a cancellation call
// has one — they have the campaign and the person. So campaign plus
// lead-or-contact is resolved with a single SOQL lookup, built through the
// shared query builder rather than by hand: both IDs are operator input and
// BuildQuery is the injection boundary.
//
// The two error returns keep the failure kinds apart. cfgErr is a configuration
// mistake (nothing to look up with, a malformed ID) and has to fail the step
// hard; lookupErr is a lookup that ran perfectly well and found nobody, which
// is data for the error port rather than a broken flow.
func resolveMemberID(instanceURL, token string, inputs []*core.Connection) (id string, cfgErr, lookupErr error) {
	if given := salesforce.OptionalString("campaign_member_id", inputs); given != "" {
		if err := salesforce.ValidateRecordID(given); err != nil {
			return "", err, nil
		}
		return given, nil, nil
	}

	campaignID := salesforce.OptionalString("campaign_id", inputs)
	leadID := salesforce.OptionalString("lead_id", inputs)
	contactID := salesforce.OptionalString("contact_id", inputs)

	if campaignID == "" || (leadID == "" && contactID == "") {
		return "", fmt.Errorf("give the Campaign Member ID, or give the Campaign ID together with the Lead ID or Contact ID so the right member can be found"), nil
	}
	if leadID != "" && contactID != "" {
		return "", fmt.Errorf("give either a Lead ID or a Contact ID, not both — a campaign member is one person"), nil
	}
	if err := salesforce.ValidateRecordID(campaignID); err != nil {
		return "", err, nil
	}

	conditions := []salesforce.Condition{{Field: "CampaignId", Operator: "=", Value: campaignID}}
	person := ""
	if leadID != "" {
		if err := salesforce.ValidateRecordID(leadID); err != nil {
			return "", err, nil
		}
		conditions = append(conditions, salesforce.Condition{Field: "LeadId", Operator: "=", Value: leadID})
		person = "lead " + leadID
	} else {
		if err := salesforce.ValidateRecordID(contactID); err != nil {
			return "", err, nil
		}
		conditions = append(conditions, salesforce.Condition{Field: "ContactId", Operator: "=", Value: contactID})
		person = "contact " + contactID
	}

	soql, err := salesforce.BuildQuery("CampaignMember", "Id", conditions, false, "", 1, true)
	if err != nil {
		return "", err, nil
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", nil, err
	}
	if record == nil {
		return "", nil, fmt.Errorf("%s is not a member of campaign %s, so there is nothing to remove", person, campaignID)
	}
	return salesforce.StringifyID(record["Id"]), nil, nil
}
