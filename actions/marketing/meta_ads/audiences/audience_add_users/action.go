// Package audience_add_users adds people to a Meta custom audience by hashed
// email or phone number.
package audience_add_users

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Audiences: Add People"
	Description  = "Add people to a Meta custom audience. Emails and phones are hashed locally before sending."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+user-plus"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "audience_id", Type: core.ConnectionTypeString, Label: "Custom Audience ID", Required: true},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Matching On", Required: true, Options: []core.ConnectionOption{
		{Name: "Email address", Value: "EMAIL"},
		{Name: "Phone number", Value: "PHONE"},
	}},
	// Values are supplied in the clear here and hashed before they leave the
	// executor — the action cannot hash what it is not given. Nothing raw is
	// ever transmitted.
	{Name: "values", Type: core.ConnectionTypeText, Label: "Email addresses or phone numbers (one per line)", Placeholder: "ada@example.com\nsam@example.com", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "audience_id", Type: core.ConnectionTypeString, Label: "Custom Audience ID"},
	{Name: "num_received", Type: core.ConnectionTypeInteger, Label: "Received by Meta"},
	{Name: "num_invalid", Type: core.ConnectionTypeInteger, Label: "Rejected by Meta"},
	{Name: "skipped_locally", Type: core.ConnectionTypeInteger, Label: "Skipped Before Sending"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	audienceID, err := meta.RequiredString("audience_id", inputs)
	if err != nil {
		return meta.ErrorResult("a custom audience ID is required"), nil
	}
	schema := meta.AudienceSchema(meta.OptionalString("schema", inputs))
	if schema != meta.SchemaEmail && schema != meta.SchemaPhone {
		return meta.ErrorResult("Matching On must be EMAIL or PHONE"), nil
	}
	rawValues, err := meta.RequiredString("values", inputs)
	if err != nil {
		return meta.ErrorResult("at least one email address or phone number is required"), nil
	}

	values := meta.SplitLines(rawValues)
	if len(values) == 0 {
		return meta.ErrorResult("no email addresses or phone numbers were supplied"), nil
	}

	// Hashing happens here, before the request is built, so there is no code
	// path in which a raw value reaches Meta.
	payload, used, skipped, err := meta.BuildAudiencePayload(schema, values)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	resp, err := meta.NewClient(token, secret).Post(flow, "/"+audienceID+"/users", url.Values{"payload": {payload}})
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	received := intOf(resp["num_received"])
	invalid := intOf(resp["num_invalid_entries"])

	summary := fmt.Sprintf("Sent %d hashed %s value(s) to audience %s — Meta received %d, rejected %d",
		used, schema, audienceID, received, invalid)
	if skipped > 0 {
		// Say this out loud. A partial upload is indistinguishable from a
		// complete one unless the shortfall is reported, and a quietly
		// half-populated audience means an ad campaign that misses people.
		summary += fmt.Sprintf(". %d entr(ies) were skipped before sending as blank or malformed", skipped)
	}
	summary += ". Meta matches audiences asynchronously, so the size will not update immediately."

	return meta.OkResult(summary, map[string]interface{}{
		"audience_id":     audienceID,
		"num_received":    received,
		"num_invalid":     invalid,
		"skipped_locally": skipped,
	}), nil
}

func intOf(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	}
	return 0
}
