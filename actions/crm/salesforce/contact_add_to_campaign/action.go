// Package crm_salesforce_contact_add_to_campaign signs a Contact up to a
// Campaign.
//
// Despite reading like an edit to the contact, this touches neither the Contact
// nor the Campaign: Salesforce models membership as its own CampaignMember
// record joining the two, which is also why adding the same person twice fails
// rather than doing nothing.
package crm_salesforce_contact_add_to_campaign

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Add Contact to Campaign"
	Description  = "Sign a contact up to a Salesforce campaign with a member status such as Sent, Registered or Responded. Optionally update their status if they are already on the list."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-group"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "0035f00000XyzAbAAJ — from the contact's Salesforce URL", Required: true},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "7015f000000XyzAAA — from the campaign's Salesforce URL", Required: true},

	// Member statuses are configured per campaign, so there is no universal
	// list: "Sent" and "Responded" exist by default, but an events team may have
	// replaced them with Registered / Attended / No Show.
	{Name: "campaign_member_status", Type: core.ConnectionTypeString, Label: "Member Status", Placeholder: "Sent, Responded, Registered… (must match this campaign's statuses)"},

	// Salesforce rejects a second membership for the same person outright. This
	// makes a re-run safe: the flow updates the existing membership instead of
	// failing, which is what an operator re-sending a list actually wants.
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

	contactID, err := salesforce.RequiredString("contact_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("contact_id is required — the 15 or 18 character ID from the contact's Salesforce URL")
	}
	if err := salesforce.ValidateRecordID(contactID); err != nil {
		return nil, err
	}
	campaignID, err := salesforce.RequiredString("campaign_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("campaign_id is required — the 15 or 18 character ID from the campaign's Salesforce URL")
	}
	if err := salesforce.ValidateRecordID(campaignID); err != nil {
		return nil, err
	}

	// Membership fields are kept apart from the two lookups because CampaignId
	// and ContactId can only be set when the membership is created — Salesforce
	// rejects them on an update.
	membership := map[string]interface{}{}
	salesforce.SetIfPresent(membership, inputs, "Status", "campaign_member_status")
	if err := salesforce.MergeAdditionalFields(membership, inputs); err != nil {
		return nil, err
	}

	updateIfMember := salesforce.OptionalBool("update_if_already_member", inputs)
	if updateIfMember {
		out, handled, err := updateExistingMembership(instanceURL, token, contactID, campaignID, membership)
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		if handled {
			return out, nil
		}
	}

	body := map[string]interface{}{"ContactId": contactID, "CampaignId": campaignID}
	for field, value := range membership {
		body[field] = value
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "CampaignMember", body)
	if err != nil {
		return salesforce.ErrorResult(explainDuplicate(err.Error(), updateIfMember)), nil
	}

	status := salesforce.OptionalString("campaign_member_status", inputs)
	if status == "" {
		status = "the campaign's default status"
	}
	return salesforce.RecordResult(id, raw, fmt.Sprintf("Added contact %s to campaign %s as %s", contactID, campaignID, status)), nil
}

// updateExistingMembership looks for an existing CampaignMember and updates it.
//
// The membership is looked up BEFORE attempting the insert rather than reacting
// to the failure, because Salesforce's duplicate-membership error carries no
// machine-readable marker worth matching on — its message is prose. One extra
// query when the option is on is a cheaper price than parsing error text.
//
// handled is false when there is no existing membership, which leaves the caller
// to create one as normal.
func updateExistingMembership(instanceURL, token, contactID, campaignID string, membership map[string]interface{}) (out map[string]interface{}, handled bool, err error) {
	soql, err := salesforce.BuildQuery("CampaignMember", "Id,Status", []salesforce.Condition{
		{Field: "CampaignId", Operator: "=", Value: campaignID},
		{Field: "ContactId", Operator: "=", Value: contactID},
	}, false, "", 1, true)
	if err != nil {
		return nil, false, err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return nil, false, err
	}
	if record == nil {
		return nil, false, nil
	}

	memberID := salesforce.StringifyID(record["Id"])
	if len(membership) == 0 {
		// Already a member and nothing to change — report that plainly instead
		// of failing, because the flow's intent (this person is on the campaign)
		// is already satisfied.
		return salesforce.RecordResult(memberID, record,
			fmt.Sprintf("Contact %s was already a member of campaign %s — nothing to change", contactID, campaignID)), true, nil
	}
	if err := salesforce.UpdateRecord(instanceURL, token, "CampaignMember", memberID, membership); err != nil {
		return nil, false, err
	}

	updated := map[string]interface{}{"Id": memberID}
	for field, value := range membership {
		updated[field] = value
	}
	return salesforce.RecordResult(memberID, updated,
		fmt.Sprintf("Contact %s was already in campaign %s — updated their membership (%s)", contactID, campaignID, strings.Join(salesforce.SortedKeys(membership), ", "))), true, nil
}

// explainDuplicate points the operator at the checkbox that would have made the
// re-run succeed. Salesforce's own wording ("that already exists in the
// campaign") is accurate but tells them nothing about what to do next.
//
// The hint is suppressed when the checkbox is ALREADY ticked: that combination
// means the pre-insert lookup found nothing and the membership appeared between
// the two calls, so telling the operator to tick a box they have ticked sends
// them looking for a setting that is not the problem.
func explainDuplicate(msg string, updateIfMember bool) string {
	if updateIfMember {
		return msg
	}
	if strings.Contains(strings.ToLower(msg), "already exists") {
		return msg + " — tick \"Update Their Status if Already a Member\" to change the existing membership instead of adding a second one"
	}
	return msg
}
