package marketing_sendgrid_block_list

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: List Blocks"
	Description  = "Retrieve the addresses on your SendGrid block list — emails the receiving server refused, for example because of an IP block or content filtering. Optionally narrow the list to a date range."
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
	{Name: "start_time", Type: core.ConnectionTypeDateTime, Label: "Start Time", Placeholder: "Only blocks created on or after this time, e.g. 2026-07-01T00:00:00Z"},
	{Name: "end_time", Type: core.ConnectionTypeDateTime, Label: "End Time", Placeholder: "Only blocks created on or before this time, e.g. 2026-07-08T23:59:59Z"},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results to return (default 100)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Tick to fetch every result page instead of just the first"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Blocks"},
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

	query := url.Values{}
	if v := sendgrid.OptionalString("start_time", inputs); v != "" {
		n, err := sendgrid.EpochSeconds(v)
		if err != nil {
			return sendgrid.ErrorResult(fmt.Sprintf("start_time: %s", err)), nil
		}
		query.Set("start_time", strconv.FormatInt(n, 10))
	}
	if v := sendgrid.OptionalString("end_time", inputs); v != "" {
		n, err := sendgrid.EpochSeconds(v)
		if err != nil {
			return sendgrid.ErrorResult(fmt.Sprintf("end_time: %s", err)), nil
		}
		query.Set("end_time", strconv.FormatInt(n, 10))
	}

	limit, _ := sendgrid.OptionalInt("limit", inputs)
	returnAll, _ := sendgrid.OptionalBoolSet("return_all", inputs)
	items, err := sendgrid.ListOffset(auth, "/v3/suppression/blocks", query, limit, returnAll)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	return sendgrid.ListResult(items, len(items), fmt.Sprintf("Retrieved %d block(s)", len(items))), nil
}
