// Package crm_salesforce_lead_convert turns a qualified Lead into an Account, a
// Contact and (usually) an Opportunity in a single Salesforce operation.
//
// This is the verb every sales-ops automation is really reaching for — "web
// enquiry comes in, rep marks it qualified, make it a real customer record" —
// and it is the one thing no REST-only integration can do: Salesforce has never
// shipped a REST endpoint for it. The alternative people fall back on is
// hand-building the three records separately and flagging the lead converted,
// which leaves duplicates behind and breaks Salesforce's own conversion
// reporting (ConvertedAccountId, ConvertedContactId, ConvertedOpportunityId are
// never populated, so the lead-to-close funnel report is simply wrong).
//
// So this action speaks SOAP. The Partner API's convertLead call takes the same
// OAuth access token as its <sessionId>, which means no extra credential, no
// separate login and no new consent — the SOAP bridge in common.go exists for
// exactly this. Conversion is atomic on Salesforce's side: either all three
// records appear and the lead is marked converted, or nothing happens.
//
// Conversion cannot be undone. Salesforce provides no "unconvert" of any kind.
package crm_salesforce_lead_convert

import (
	"encoding/xml"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Convert Lead"
	Description  = "Turn a qualified lead into an account, a contact and an opportunity in one step. This cannot be undone, so run it only once the lead is genuinely qualified."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+diagram-project"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "lead_id", Type: core.ConnectionTypeString, Label: "Lead ID", Placeholder: "00Q5f000004XyzAEAS", Required: true},

	// This must be a lead status your administrator has ticked "Converted" in
	// Setup — an ordinary status is rejected. Most orgs ship with exactly one,
	// named "Closed - Converted".
	{Name: "converted_status", Type: core.ConnectionTypeString, Label: "Converted Status", Placeholder: "Closed - Converted", Required: true},

	// Leaving both of these blank is the normal case: Salesforce creates a new
	// account and contact from the lead's own company and name. Fill one in
	// only when the customer already exists and you want to attach to it,
	// otherwise you get a duplicate account for an existing customer.
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Attach to Existing Account", Placeholder: "0015f00000XyzAAA — leave blank to create a new account"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Attach to Existing Contact", Placeholder: "0035f00000XyzAAA — leave blank to create a new contact"},

	{Name: "opportunity_name", Type: core.ConnectionTypeString, Label: "Opportunity Name", Placeholder: "Acme Ltd — new enquiry (blank names it after the company)"},
	{Name: "do_not_create_opportunity", Type: core.ConnectionTypeBoolean, Label: "Do Not Create an Opportunity"},

	// Overwriting the lead source rewrites the EXISTING contact's Lead Source
	// with the lead's. Salesforce only allows it when a contact is supplied.
	{Name: "overwrite_lead_source", Type: core.ConnectionTypeBoolean, Label: "Overwrite the Contact's Lead Source"},
	{Name: "send_notification_email", Type: core.ConnectionTypeBoolean, Label: "Email the New Owner"},

	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assign the New Records To", Placeholder: "Salesforce user ID, e.g. 0055f000004XyzAAB (defaults to the lead's owner)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Converted Lead ID"},
	// The three new record IDs live in here — result.accountId, result.contactId
	// and result.opportunityId — which is what a downstream node chains off.
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Converted Records"},
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
		return nil, fmt.Errorf("lead_id is required — the Salesforce record ID of the lead to convert, e.g. 00Q5f000004XyzAEAS")
	}
	if err := salesforce.ValidateRecordID(leadID); err != nil {
		return nil, err
	}

	convertedStatus, err := salesforce.RequiredString("converted_status", inputs)
	if err != nil {
		return nil, fmt.Errorf("converted_status is required — the lead status your administrator has marked as Converted, usually \"Closed - Converted\"")
	}

	accountID := salesforce.OptionalString("account_id", inputs)
	if accountID != "" {
		if err := salesforce.ValidateRecordID(accountID); err != nil {
			return nil, fmt.Errorf("account_id: %w", err)
		}
	}
	contactID := salesforce.OptionalString("contact_id", inputs)
	if contactID != "" {
		if err := salesforce.ValidateRecordID(contactID); err != nil {
			return nil, fmt.Errorf("contact_id: %w", err)
		}
	}
	ownerID := salesforce.OptionalString("owner_id", inputs)
	if ownerID != "" {
		if err := salesforce.ValidateRecordID(ownerID); err != nil {
			return nil, fmt.Errorf("owner_id: %w", err)
		}
	}

	opportunityName := salesforce.OptionalString("opportunity_name", inputs)
	doNotCreateOpportunity := salesforce.OptionalBool("do_not_create_opportunity", inputs)
	overwriteLeadSource := salesforce.OptionalBool("overwrite_lead_source", inputs)
	sendNotificationEmail := salesforce.OptionalBool("send_notification_email", inputs)

	// Two combinations Salesforce refuses, caught here because the server-side
	// message for both is a bare status code that gives no clue which of the
	// operator's two settings is the one to change.
	if doNotCreateOpportunity && opportunityName != "" {
		return nil, fmt.Errorf("you have asked not to create an opportunity but also given it a name — clear the opportunity name, or untick \"Do Not Create an Opportunity\"")
	}
	if overwriteLeadSource && contactID == "" {
		return nil, fmt.Errorf("\"Overwrite the Contact's Lead Source\" only applies when you attach to an existing contact — set contact_id, or untick it")
	}

	body, err := salesforce.SOAPCall(instanceURL, token, buildConvertLead(convertLeadRequest{
		AccountID:              accountID,
		ContactID:              contactID,
		ConvertedStatus:        convertedStatus,
		DoNotCreateOpportunity: doNotCreateOpportunity,
		LeadID:                 leadID,
		OpportunityName:        opportunityName,
		OverwriteLeadSource:    overwriteLeadSource,
		OwnerID:                ownerID,
		SendNotificationEmail:  sendNotificationEmail,
	}))
	if err != nil {
		// Salesforce refused (wrong status, already converted, no permission).
		// A provider failure, so it lands on the error port as data.
		return salesforce.ErrorResult(err.Error()), nil
	}

	var env convertLeadResponse
	if err := xml.Unmarshal(body, &env); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("Salesforce returned a conversion response that could not be read: %v", err)), nil
	}

	// convertLead can answer HTTP 200 with success=false and a per-record error
	// list — a soft refusal that is easy to mistake for a successful run if the
	// only thing checked is the status code.
	if !env.Result.Success {
		return salesforce.ErrorResult(formatConvertErrors(env.Result.Errors, leadID)), nil
	}

	converted := map[string]interface{}{
		"leadId":        env.Result.LeadID,
		"accountId":     env.Result.AccountID,
		"contactId":     env.Result.ContactID,
		"opportunityId": env.Result.OpportunityID,
		"success":       true,
	}
	// The lead's own ID is echoed back, but fall back to the one that was sent
	// so downstream never gets an empty ID from a terse response.
	if env.Result.LeadID == "" {
		converted["leadId"] = leadID
	}

	return salesforce.RecordResult(leadID, converted, summarise(env.Result)), nil
}

// convertLeadRequest is one <leadConverts> block. Its field order mirrors the
// Partner WSDL's LeadConvert sequence, which SOAP enforces strictly — elements
// out of order are rejected outright, so the order below is not cosmetic.
type convertLeadRequest struct {
	AccountID              string
	ContactID              string
	ConvertedStatus        string
	DoNotCreateOpportunity bool
	LeadID                 string
	OpportunityName        string
	OverwriteLeadSource    bool
	OwnerID                string
	SendNotificationEmail  bool
}

// buildConvertLead assembles the SOAP operation element.
//
// Every operator-supplied value goes through XMLEscape: the body is assembled as
// a string, so this is the same injection boundary EscapeSOQLString guards for
// queries. The optional ID elements are omitted rather than sent empty, because
// an empty <accountId/> is read as "attach to the account with a blank ID" and
// fails; the three booleans are always sent, as the WSDL marks them mandatory.
func buildConvertLead(r convertLeadRequest) string {
	var b strings.Builder
	b.WriteString("<urn:convertLead><urn:leadConverts>")
	writeOptional(&b, "accountId", r.AccountID)
	writeOptional(&b, "contactId", r.ContactID)
	writeElement(&b, "convertedStatus", r.ConvertedStatus)
	writeBool(&b, "doNotCreateOpportunity", r.DoNotCreateOpportunity)
	writeElement(&b, "leadId", r.LeadID)
	writeOptional(&b, "opportunityName", r.OpportunityName)
	writeBool(&b, "overwriteLeadSource", r.OverwriteLeadSource)
	writeOptional(&b, "ownerId", r.OwnerID)
	writeBool(&b, "sendNotificationEmail", r.SendNotificationEmail)
	b.WriteString("</urn:leadConverts></urn:convertLead>")
	return b.String()
}

func writeElement(b *strings.Builder, name, value string) {
	b.WriteString("<urn:")
	b.WriteString(name)
	b.WriteString(">")
	b.WriteString(salesforce.XMLEscape(value))
	b.WriteString("</urn:")
	b.WriteString(name)
	b.WriteString(">")
}

func writeOptional(b *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	writeElement(b, name, value)
}

func writeBool(b *strings.Builder, name string, value bool) {
	if value {
		writeElement(b, name, "true")
		return
	}
	writeElement(b, name, "false")
}

// convertLeadResponse models the Partner API reply. Go matches XML element local
// names, so the soapenv/urn prefixes on the wire need no namespace declarations
// here — the same approach common.go's soapFault takes.
type convertLeadResponse struct {
	Result convertLeadResult `xml:"Body>convertLeadResponse>result"`
}

// convertLeadResult is the LeadConvertResult element: the three IDs the
// conversion produced, plus the soft-failure channel.
type convertLeadResult struct {
	AccountID     string             `xml:"accountId"`
	ContactID     string             `xml:"contactId"`
	LeadID        string             `xml:"leadId"`
	OpportunityID string             `xml:"opportunityId"`
	Success       bool               `xml:"success"`
	Errors        []convertLeadError `xml:"errors"`
}

type convertLeadError struct {
	Fields     []string `xml:"fields"`
	Message    string   `xml:"message"`
	StatusCode string   `xml:"statusCode"`
}

// formatConvertErrors renders a soft refusal into one readable line, and adds
// the "why" for the two codes that account for nearly every failed conversion.
func formatConvertErrors(errs []convertLeadError, leadID string) string {
	if len(errs) == 0 {
		return fmt.Sprintf("Salesforce refused to convert lead %s but gave no reason — check that the lead is not already converted and that the connected user has Convert Leads permission", leadID)
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		msg := strings.TrimSpace(e.Message)
		switch e.StatusCode {
		case "INVALID_STATUS":
			msg = "that status is not marked as a Converted status in your Salesforce setup — ask your administrator which one is (" + msg + ")"
		case "CANNOT_UPDATE_CONVERTED_LEAD":
			msg = "this lead has already been converted, and Salesforce cannot convert it twice (" + msg + ")"
		}
		if msg == "" {
			msg = e.StatusCode
		}
		if len(e.Fields) > 0 {
			msg += " — field(s): " + strings.Join(e.Fields, ", ")
		}
		parts = append(parts, msg)
	}
	return strings.Join(parts, "; ")
}

// summarise names the records that were created so the run history reads like
// something that happened, not like a list of IDs.
func summarise(result convertLeadResult) string {
	created := make([]string, 0, 3)
	if result.AccountID != "" {
		created = append(created, "account "+result.AccountID)
	}
	if result.ContactID != "" {
		created = append(created, "contact "+result.ContactID)
	}
	if result.OpportunityID != "" {
		created = append(created, "opportunity "+result.OpportunityID)
	}
	if len(created) == 0 {
		return "Converted the lead"
	}
	return "Converted the lead into " + strings.Join(created, ", ")
}
