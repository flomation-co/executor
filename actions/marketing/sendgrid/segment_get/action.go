package marketing_sendgrid_segment_get

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Get Segment"
	Description  = "Look up one segment in SendGrid Marketing — its query, contact count, and a sample of the contacts currently in it."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+eye"
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
	{Name: "segment_id", Type: core.ConnectionTypeString, Label: "Segment", Placeholder: "The segment to fetch — see \"SendGrid: List Segments\"", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Segment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Segment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	segmentID, err := sendgrid.RequiredString("segment_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/marketing/segments/2.0/"+url.PathEscape(segmentID), nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid response shape"), nil
	}
	label := sendgrid.StringifyID(obj["id"])
	if name, _ := obj["name"].(string); name != "" {
		label = name
	}
	return sendgrid.ResourceResult(sendgrid.StringifyID(obj["id"]), obj, fmt.Sprintf("Retrieved segment %q", label)), nil
}
