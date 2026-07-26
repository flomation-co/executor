// Package crm_salesforce_user_get_current identifies the connected Salesforce
// login.
//
// This is the node's connection probe: the cheapest possible "is this working,
// and which org and user am I actually pointed at". It answers the question that
// causes most first-run confusion — a token minted against the sandbox while the
// flow was meant to hit production authenticates perfectly and then quietly
// writes to the wrong org.
//
// It is also the natural first step in any flow that stamps ownership, since
// every record this connection creates will be owned by the user it names.
package crm_salesforce_user_get_current

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Current User"
	Description  = "Check the Salesforce connection and see who it is signed in as — the username, email, org and time zone the rest of your flow will run under."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+circle-check"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// userInfoPath is Salesforce's OpenID Connect identity endpoint.
//
// It sits directly under the instance, NOT under /services/data/vNN, which is
// exactly what makes it the right connection probe: it answers even when the
// REST API version this node pins has not been rolled out to the org yet, so a
// failure here means the token is genuinely bad rather than the version being
// wrong. That is also why it goes through ExecuteAbsolute — ExecuteAPI would
// prefix the version root.
const userInfoPath = "/services/oauth2/userinfo"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Connected User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := salesforce.ExecuteAbsolute(instanceURL, token, http.MethodGet, userInfoPath, nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		// The whole point of this action is to report a broken connection as
		// data, so an expired or revoked token lands on the error port for a
		// downstream branch to handle rather than stopping the flow dead.
		return salesforce.ErrorResult(err.Error()), nil
	}

	info := map[string]interface{}{}
	if len(bytes.TrimSpace(resp.Body)) > 0 {
		if err := json.Unmarshal(resp.Body, &info); err != nil {
			return salesforce.ErrorResult(fmt.Sprintf("Salesforce returned a response that could not be read: %v — this usually means the Instance URL points at a login page rather than your org", err)), nil
		}
	}

	// The identity endpoint names its fields the OpenID Connect way, not the
	// Salesforce way: user_id rather than Id, preferred_username rather than
	// Username. Nothing downstream should have to know that, so the record ID is
	// lifted into the standard id output.
	userID := salesforce.StringifyID(info["user_id"])

	return salesforce.RecordResult(userID, info, describeConnection(info, userID)), nil
}

// describeConnection renders the one line an operator reads to confirm the
// connection is pointed at the org and login they expected — the usual mistake
// is a token for the sandbox when the flow is meant to run against production.
func describeConnection(info map[string]interface{}, userID string) string {
	name := textField(info, "name")
	username := textField(info, "preferred_username")
	if username == "" {
		username = textField(info, "email")
	}

	who := strings.TrimSpace(name)
	switch {
	case who != "" && username != "":
		who = fmt.Sprintf("%s (%s)", who, username)
	case who == "" && username != "":
		who = username
	case who == "":
		who = userID
	}

	summary := "Connected to Salesforce as " + who
	if org := textField(info, "organization_id"); org != "" {
		summary += ", org " + org
	}
	// active is false for a deactivated login whose token has not yet expired —
	// a connection that authenticates but will fail on the first real write.
	if active, ok := info["active"].(bool); ok && !active {
		summary += " — WARNING: this Salesforce login is deactivated"
	}
	return summary
}

// textField reads a string field from the identity payload, tolerating the
// nulls Salesforce returns for anything the user has not filled in.
func textField(info map[string]interface{}, key string) string {
	v, ok := info[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}
