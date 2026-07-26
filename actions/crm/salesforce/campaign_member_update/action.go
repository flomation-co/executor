package crm_salesforce_campaign_member_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Campaign Member"
	Description  = "Move someone along in a Salesforce campaign — Registered to Attended, or Sent to Responded. Give the campaign member's ID, or just the campaign plus the lead or contact and they will be found for you."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+circle-check"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "campaign_member_id", Type: core.ConnectionTypeString, Label: "Campaign Member ID", Placeholder: "00v... — or leave blank and fill in the three boxes below instead"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "701... — used to find the member when you have not got their member ID"},
	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q... — the lead to find within that campaign"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "003... — the contact to find within that campaign"},
	{Name: "campaign_member_status", Type: core.ConnectionTypeString, Label: "Member Status", Placeholder: "The status to move them to, e.g. Responded — must match your org's Campaign Member Status list for that campaign"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Description":"Arrived at 09:15","Table_Number__c":"12"} — a null empties that field`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign Member ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Applied Changes"},
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

	// Only the status and any custom fields are worth exposing here: Salesforce
	// will not let CampaignId, LeadId or ContactId change on an existing member.
	// Moving somebody to a different campaign means removing them and adding
	// them again, which is what the delete and add actions are for.
	body := map[string]interface{}{}
	status := salesforce.OptionalString("campaign_member_status", inputs)
	salesforce.SetIfPresent(body, inputs, "Status", "campaign_member_status")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to change — set a Member Status, or put the fields you want changed in Additional Fields")
	}

	// The statuses a campaign accepts are defined per campaign (Setup ▸ Campaign
	// Member Statuses), so a perfectly sensible word such as "Attended" is still
	// rejected by a campaign whose admin never added it. That comes back as a
	// provider error and lands on the error port.
	if err := salesforce.UpdateRecord(instanceURL, token, "CampaignMember", memberID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers a successful update with 204 No Content, so there is no
	// record to hand back. Return the member ID plus the changes that were
	// applied — an empty result would strand anything wired to this step.
	applied := map[string]interface{}{"Id": memberID}
	for k, v := range body {
		applied[k] = v
	}
	summary := fmt.Sprintf("Updated campaign member %s (%s)", memberID, strings.Join(salesforce.SortedKeys(body), ", "))
	if status != "" {
		summary = fmt.Sprintf("Moved campaign member %s to %q", memberID, status)
	}
	return salesforce.RecordResult(memberID, applied, summary), nil
}

// resolveMemberID works out which CampaignMember record to change.
//
// The operator can give its ID directly, but nobody working a reception desk
// has one — they have the campaign and the person in front of them. So campaign
// plus lead-or-contact is resolved with a single SOQL lookup, built through the
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
		return "", nil, fmt.Errorf("%s is not a member of campaign %s, so there is nothing to update — add them to the campaign first", person, campaignID)
	}
	return salesforce.StringifyID(record["Id"]), nil, nil
}
