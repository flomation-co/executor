package crm_salesforce_campaign_member_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Campaign Member"
	Description  = "Sign a lead or a contact up to a Salesforce campaign and set where they have got to — Sent, Responded, Registered, whatever your campaign uses. Give either a Lead or a Contact, not both."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "701... — the campaign to add them to", Required: true},
	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q... — use this for someone who is still a lead"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "003... — use this for someone who is already a contact"},
	{Name: "campaign_member_status", Type: core.ConnectionTypeString, Label: "Member Status", Placeholder: "Sent or Responded, unless your admin set up others such as Registered or Attended"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"Description":"Signed up on the website","Table_Number__c":"12"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign Member ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Salesforce key prefixes are fixed for standard objects in every org: 00Q is a
// Lead, 003 a Contact, 701 a Campaign. Checking them turns Salesforce's opaque
// INVALID_CROSS_REFERENCE_KEY into a message that names the box the ID belongs
// in. Only an ID that is definitely the WRONG standard object is rejected — an
// unfamiliar prefix is left for Salesforce to judge, so this can never block a
// legitimate ID it has not been taught about.
const (
	leadPrefix    = "00Q"
	contactPrefix = "003"
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	campaignID := salesforce.OptionalString("campaign_id", inputs)
	if err := salesforce.ValidateRecordID(campaignID); err != nil {
		return nil, err
	}
	if strings.HasPrefix(campaignID, leadPrefix) || strings.HasPrefix(campaignID, contactPrefix) {
		return nil, fmt.Errorf("%q is a person, not a campaign — Salesforce campaign IDs start 701 and can be copied from the end of the campaign's web address", campaignID)
	}

	// A CampaignMember points at exactly one person: a Lead or a Contact, never
	// both and never neither. Salesforce rejects the other combinations, but it
	// does so with a message about field integrity that means nothing to an
	// operator at a reception desk.
	leadID := salesforce.OptionalString("lead_id", inputs)
	contactID := salesforce.OptionalString("contact_id", inputs)
	switch {
	case leadID == "" && contactID == "":
		return nil, fmt.Errorf("give either a Lead ID or a Contact ID — a campaign member has to be one person")
	case leadID != "" && contactID != "":
		return nil, fmt.Errorf("give either a Lead ID or a Contact ID, not both — if the lead has already been converted, use the contact")
	}

	body := map[string]interface{}{"CampaignId": campaignID}
	person := ""
	if leadID != "" {
		if err := salesforce.ValidateRecordID(leadID); err != nil {
			return nil, err
		}
		if strings.HasPrefix(leadID, contactPrefix) {
			return nil, fmt.Errorf("%q is a Contact ID (they start 003) — put it in the Contact ID box instead", leadID)
		}
		body["LeadId"] = leadID
		person = "lead " + leadID
	} else {
		if err := salesforce.ValidateRecordID(contactID); err != nil {
			return nil, err
		}
		if strings.HasPrefix(contactID, leadPrefix) {
			return nil, fmt.Errorf("%q is a Lead ID (they start 00Q) — put it in the Lead ID box instead", contactID)
		}
		body["ContactId"] = contactID
		person = "contact " + contactID
	}

	// Status is left off entirely when blank so Salesforce applies the
	// campaign's own default member status. The statuses a campaign accepts are
	// defined per campaign (Setup ▸ Campaign Member Statuses), so a value that
	// reads perfectly well — "Attended" — is still rejected by a campaign whose
	// admin never added it.
	status := salesforce.OptionalString("campaign_member_status", inputs)
	salesforce.SetIfPresent(body, inputs, "Status", "campaign_member_status")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	// A person already signed up to this campaign comes back as a duplicate
	// error from Salesforce. That is a provider answer, not a configuration
	// mistake, so it lands on the error port where a flow can branch on it.
	id, raw, err := salesforce.CreateRecord(instanceURL, token, "CampaignMember", body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Added %s to campaign %s (%s)", person, campaignID, id)
	if status != "" {
		summary = fmt.Sprintf("Added %s to campaign %s as %q (%s)", person, campaignID, status, id)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}
