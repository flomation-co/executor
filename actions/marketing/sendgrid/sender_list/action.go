package marketing_sendgrid_sender_list

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: List Verified Senders"
	Description  = "Retrieve the verified single senders on your SendGrid account — the From addresses you can send from. A domain you have authenticated additionally allows any From address on that domain."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+list"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results — leave blank for all senders"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Verified Senders"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/verified_senders", nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	items := extractSenders(result)
	if limit, set := sendgrid.OptionalInt("limit", inputs); set && limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return sendgrid.ListResult(items, len(items), fmt.Sprintf("Retrieved %d verified sender(s)", len(items))), nil
}

// extractSenders unwraps the verified-senders envelope ({"results": [...]}),
// tolerating the singular "result" key or a bare top-level array defensively.
func extractSenders(result interface{}) []interface{} {
	switch v := result.(type) {
	case []interface{}:
		return v
	case map[string]interface{}:
		for _, key := range []string{"results", "result"} {
			if arr, ok := v[key].([]interface{}); ok {
				return arr
			}
		}
	}
	return []interface{}{}
}
