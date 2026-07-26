// Package crm_salesforce_quick_action_run runs one of the org's quick actions.
//
// A quick action is the button a Salesforce user clicks — "New Contact", "Log a
// Call", "New Case" — and the admin has already decided what it does: which
// record type it uses, which fields it shows, what it defaults them to. Running
// one from an automation therefore produces a record shaped exactly the way the
// org expects, which is a great deal safer for a non-technical operator than
// typing field names into a create action and hoping the record type is right.
package crm_salesforce_quick_action_run

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Run Quick Action"
	Description  = "Create or update a record using one of your Salesforce quick actions, so it comes out exactly the way your administrator set the action up."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Account — leave blank for an org-wide quick action"},
	{Name: "quick_action_name", Type: core.ConnectionTypeString, Label: "Quick Action", Placeholder: "LogACall — the quick action's API name", Required: true},
	{Name: "context_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "0015f00000AbCdEAAV — the record the action is being run from"},
	{Name: "field_name", Type: core.ConnectionTypeString, Label: "Field to Set", Placeholder: "Subject"},
	{Name: "field_value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "Called the customer back"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "More Fields (JSON)", Placeholder: `{"Subject":"Called the customer back","Description":"Agreed a follow-up for Friday"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// quickActionResponse is the invoke envelope: the same {id, success, errors}
// shape a record create returns, plus the Chatter feed items the action posted
// and the context record it ran against.
//
// Errors is deliberately untyped: this endpoint returns the usual
// {message, statusCode} objects for a validation failure but plain strings for
// some layout errors, and a typed slice would fail to decode the whole response
// on the string form — losing the ID and the success flag with it.
type quickActionResponse struct {
	ID          string        `json:"id"`
	Success     bool          `json:"success"`
	Created     bool          `json:"created"`
	ContextID   string        `json:"contextId"`
	FeedItemIDs []string      `json:"feedItemIds"`
	Errors      []interface{} `json:"errors"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	actionName, err := salesforce.RequiredString("quick_action_name", inputs)
	if err != nil {
		return nil, err
	}
	// The action name goes into the request path. It is validated with the
	// FIELD name rule rather than the object one because object-specific quick
	// actions are addressed with a dotted name (Account.NewContact) and only
	// the field pattern allows the dot; both patterns reject everything that
	// could break out of the path.
	safeName, err := salesforce.ValidateSOQLFieldName(actionName)
	if err != nil {
		return nil, fmt.Errorf("%q is not a Salesforce quick action name — copy it from Get Many Quick Actions (letters, numbers, underscores and an optional object prefix, e.g. LogACall or Account.NewContact)", actionName)
	}

	path := "/quickActions/" + url.PathEscape(safeName)
	target := "org-wide"
	if object := salesforce.OptionalString("object", inputs); object != "" {
		obj, err := salesforce.ValidateSOQLObjectName(object)
		if err != nil {
			return nil, err
		}
		path = "/sobjects/" + obj + "/quickActions/" + url.PathEscape(safeName)
		target = obj
	}

	fields, err := buildFields(inputs)
	if err != nil {
		return nil, err
	}

	// Salesforce wraps the field values in a "record" object and takes the
	// record the action is being run FROM as a sibling "contextId" — that is
	// how a "New Contact" action on an Account knows which Account to hang the
	// contact off, and how "Log a Call" finds the record to log against.
	contextID := salesforce.OptionalString("context_id", inputs)
	if contextID != "" {
		if err := salesforce.ValidateRecordID(contextID); err != nil {
			return nil, err
		}
	}
	body := map[string]interface{}{"record": fields}
	if contextID != "" {
		body["contextId"] = contextID
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodPost, path, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// An empty body is a SUCCESS, not a parse failure: a quick action bound to
	// a Visualforce page or a Chatter post answers 204 No Content with nothing
	// at all, and decoding that as JSON fails with "unexpected end of JSON
	// input" — an invented error on a call that worked. Echo the record the
	// action ran against and the values written instead, the same way every
	// 204 update in this package does, so the next node still has something to
	// chain off.
	if len(strings.TrimSpace(string(resp.Body))) == 0 {
		record := map[string]interface{}{}
		for field, value := range fields {
			record[field] = value
		}
		summary := fmt.Sprintf("Ran the %s quick action (%s) — Salesforce returned no record", safeName, target)
		if contextID != "" {
			record["Id"] = contextID
			summary = fmt.Sprintf("Ran the %s quick action (%s) against %s — Salesforce returned no record", safeName, target, contextID)
		}
		return salesforce.RecordResult(contextID, record, summary), nil
	}

	var qa quickActionResponse
	if err := json.Unmarshal(resp.Body, &qa); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce quick action response: %v", err)), nil
	}
	// A quick action can answer 200 while reporting success:false — a layout
	// validation rule rejecting a value looks exactly like this — so the flag
	// is checked rather than trusting the status code alone.
	if !qa.Success && len(qa.Errors) > 0 {
		return salesforce.ErrorResult(fmt.Sprintf("Salesforce rejected the %s quick action: %s", safeName, formatErrors(qa.Errors))), nil
	}

	raw, err := decodeRaw(resp.Body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	verb := "Updated"
	if qa.Created {
		verb = "Created"
	}
	summary := fmt.Sprintf("%s a record with the %s quick action (%s)", verb, safeName, target)
	if qa.ID != "" {
		summary = fmt.Sprintf("%s %s with the %s quick action (%s)", verb, qa.ID, safeName, target)
	}
	return salesforce.RecordResult(qa.ID, raw, summary), nil
}

// buildFields assembles the values the quick action's layout expects.
//
// The single field pair covers the everyday case (one subject line, one status)
// without anyone meeting JSON; additional_fields is the escape hatch that makes
// the action usable against ANY layout, which matters because a quick action's
// field list is whatever the admin dragged onto it — no fixed set of inputs
// could ever cover it. The named pair is applied last so it wins on a clash.
func buildFields(inputs []*core.Connection) (map[string]interface{}, error) {
	fields := map[string]interface{}{}
	if err := salesforce.MergeAdditionalFields(fields, inputs); err != nil {
		return nil, err
	}

	name := salesforce.OptionalString("field_name", inputs)
	value := salesforce.OptionalString("field_value", inputs)
	if name == "" && value != "" {
		return nil, fmt.Errorf("field_value was set without field_name — Salesforce needs the name of the field to fill in, e.g. Subject")
	}
	if name != "" {
		field, err := salesforce.ValidateSOQLFieldName(name)
		if err != nil {
			return nil, err
		}
		fields[field] = value
	}
	return fields, nil
}

// decodeRaw returns the response as a plain map for the result output. The
// empty case is handled in Execute (a 204 from a Visualforce or Chatter quick
// action); the guard is kept here so this helper cannot invent a parse error
// out of a body that simply is not there.
func decodeRaw(body []byte) (map[string]interface{}, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]interface{}{}, nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse the Salesforce quick action response: %w", err)
	}
	return raw, nil
}

// formatErrors renders the quick action's error array into one readable line,
// accepting both the {message, statusCode} object form and the bare strings
// this endpoint returns for some layout errors.
func formatErrors(errs []interface{}) string {
	parts := make([]string, 0, len(errs))
	for _, raw := range errs {
		e, ok := raw.(map[string]interface{})
		if !ok {
			if s := strings.TrimSpace(fmt.Sprintf("%v", raw)); s != "" {
				parts = append(parts, s)
			}
			continue
		}
		msg, _ := e["message"].(string)
		code, _ := e["statusCode"].(string)
		if code == "" {
			code, _ = e["errorCode"].(string)
		}
		switch {
		case msg != "" && code != "":
			parts = append(parts, msg+" ("+code+")")
		case msg != "":
			parts = append(parts, msg)
		case code != "":
			parts = append(parts, code)
		default:
			parts = append(parts, fmt.Sprintf("%v", e))
		}
	}
	return strings.Join(parts, "; ")
}
