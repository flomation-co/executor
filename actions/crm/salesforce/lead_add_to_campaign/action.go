// Package crm_salesforce_lead_add_to_campaign records that a Lead is taking
// part in a Campaign.
//
// Salesforce models this as a junction record — a CampaignMember row joining the
// lead to the campaign — so nothing on the Lead itself changes. That is why the
// action creates a CampaignMember rather than touching /sobjects/Lead, and why
// the ID it returns is the membership, not the lead.
//
// A CampaignMember carries exactly one of LeadId or ContactId, and Salesforce
// refuses a second membership for the same lead and campaign pair. That is the
// right default — nobody wants a duplicate attendee list — but it makes a flow
// fail the moment it runs twice over an overlapping list, which is the normal
// state of affairs for a scheduled import. "Update Their Status if Already a
// Member" turns that failure into the thing the operator actually meant.
package crm_salesforce_lead_add_to_campaign

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Lead to Campaign"
	Description  = "Sign a lead up to a campaign — an event, a webinar, a mailshot — and set how they are taking part, such as Sent, Registered or Responded."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-group"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q5f000004XyzAEAS — from the lead's Salesforce URL", Required: true},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "7015f000000XyzAAA — from the campaign's Salesforce URL", Required: true},

	// Member Status is a per-campaign picklist the admin configures on the
	// campaign itself, and which value counts as "responded" is part of that
	// setup. Leaving it blank takes the campaign's own default.
	{Name: "campaign_member_status", Type: core.ConnectionTypeString, Label: "Member Status", Placeholder: "Sent, Responded, Registered — must match this campaign's Member Status list"},

	{Name: "update_if_already_member", Type: core.ConnectionTypeBoolean, Label: "Update Their Status if Already a Member"},

	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: `{"HasResponded":true,"Custom_Field__c":"value"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign Member ID"},
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

	leadID, err := salesforce.RequiredString("lead_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("lead_id is required — the Salesforce record ID of the lead to add, e.g. 00Q5f000004XyzAEAS")
	}
	if err := salesforce.ValidateRecordID(leadID); err != nil {
		return nil, err
	}

	campaignID, err := salesforce.RequiredString("campaign_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("campaign_id is required — the Salesforce record ID of the campaign, e.g. 7015f000000XyzAAA")
	}
	if err := salesforce.ValidateRecordID(campaignID); err != nil {
		return nil, err
	}

	// Status is omitted when blank so the campaign's configured default wins —
	// sending an empty string would be rejected as an invalid picklist value.
	fields := map[string]interface{}{}
	salesforce.SetIfPresent(fields, inputs, "Status", "campaign_member_status")
	if err := salesforce.MergeAdditionalFields(fields, inputs); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"LeadId":     leadID,
		"CampaignId": campaignID,
	}
	for k, v := range fields {
		body[k] = v
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "CampaignMember", body)
	if err == nil {
		return salesforce.RecordResult(id, raw, describeAdded(inputs, leadID, campaignID)), nil
	}

	if !isDuplicate(err.Error()) {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if !salesforce.OptionalBool("update_if_already_member", inputs) {
		return salesforce.ErrorResult(fmt.Sprintf("lead %s is already a member of campaign %s — Salesforce allows one membership per lead per campaign. Tick \"Update Their Status if Already a Member\" to change their status instead of failing (%s)", leadID, campaignID, err.Error())), nil
	}

	return updateExistingMember(instanceURL, token, leadID, campaignID, fields)
}

// updateExistingMember handles the "they are already on the list" path: find the
// membership Salesforce refused to duplicate and change it instead.
//
// Only the supplied fields are PATCHed — LeadId and CampaignId are deliberately
// left out, because they are the pair that identifies the membership and
// Salesforce rejects any attempt to move a member to a different campaign.
func updateExistingMember(instanceURL, token, leadID, campaignID string, fields map[string]interface{}) (map[string]interface{}, error) {
	soql, err := salesforce.BuildQuery("CampaignMember", "Id,Status", []salesforce.Condition{
		{Field: "CampaignId", Operator: "=", Value: campaignID},
		{Field: "LeadId", Operator: "=", Value: leadID},
	}, false, "", 1, true)
	if err != nil {
		return nil, err
	}

	member, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if member == nil {
		// Salesforce said duplicate but will not show it to us. The usual cause
		// is field-level security on CampaignMember for the connected user, so
		// say that rather than leaving a contradiction in the run history.
		return salesforce.ErrorResult(fmt.Sprintf("Salesforce says lead %s is already in campaign %s, but the connected user cannot see that membership — check their access to Campaign Members", leadID, campaignID)), nil
	}
	memberID := salesforce.StringifyID(member["Id"])

	if len(fields) == 0 {
		// Nothing to change: they are on the list and no new status was given.
		// That is a success, not a no-op failure — the flow's intent is met.
		return salesforce.RecordResult(memberID, member, fmt.Sprintf("Lead %s was already a member of campaign %s; nothing to change", leadID, campaignID)), nil
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "CampaignMember", memberID, fields); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// 204 No Content on the PATCH, so report what was written.
	updated := map[string]interface{}{"Id": memberID, "LeadId": leadID, "CampaignId": campaignID}
	for k, v := range fields {
		updated[k] = v
	}
	summary := fmt.Sprintf("Lead %s was already in campaign %s — updated their membership (%s)", leadID, campaignID, strings.Join(salesforce.SortedKeys(fields), ", "))
	return salesforce.RecordResult(memberID, updated, summary), nil
}

// isDuplicate spots Salesforce's refusal to add the same lead to the same
// campaign twice. The code is matched as well as the prose because the message
// text ("duplicate value found: unknown duplicates value on record with id: …")
// is not something to depend on.
func isDuplicate(msg string) bool {
	return strings.Contains(msg, "DUPLICATE_VALUE") || strings.Contains(strings.ToLower(msg), "duplicate value")
}

// describeAdded builds the success line for a fresh membership.
func describeAdded(inputs []*core.Connection, leadID, campaignID string) string {
	summary := fmt.Sprintf("Added lead %s to campaign %s", leadID, campaignID)
	if status := salesforce.OptionalString("campaign_member_status", inputs); status != "" {
		summary += fmt.Sprintf(" as %s", status)
	}
	return summary
}
