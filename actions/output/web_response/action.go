package web_response

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Web Response"
	Description  = "Set the HTTP response (body, status code, content type, headers) returned by a Web Trigger flow"
	Website      = "https://www.flomation.co"
	Icon         = "reply"
	Date         = "13/07/2026"
	Type         = core.ActionTypeOutput
)

// WebResponseKey is the reserved flow-output key the Web Response is captured
// under. The API reads it from the execution's outputs to build the HTTP
// response for a hanging Web Trigger invoke. Double-underscored so it never
// collides with a user's Set Output value.
const WebResponseKey = "__web_response__"

var Inputs = [...]core.Connection{
	{Name: "body", Type: core.ConnectionTypeText, Label: "Response Body"},
	{Name: "status_code", Type: core.ConnectionTypeInteger, Label: "Status Code", Placeholder: "200"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type", Placeholder: "application/json"},
	{Name: "headers", Type: core.ConnectionTypeText, Label: "Headers (JSON object)", Placeholder: `{"Location":"/x"}`},
}

var Outputs = [...]core.Connection{
	{Name: "set", Type: core.ConnectionTypeBoolean, Label: "Set"},
}

// Execute captures the response fields onto the flow output under WebResponseKey
// (a SetOutput-style capture). Only values set via flow.SetOutput reach the
// flow's final result, so the API can read this after the flow terminates.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	resp := map[string]interface{}{}
	if body := core.FindConnection("body", inputs); body != nil && body.Value != nil {
		resp["body"] = body.Value
	}
	if status := core.FindConnection("status_code", inputs); status != nil && status.Value != nil {
		resp["status_code"] = status.Value
	}
	if ct := core.FindConnection("content_type", inputs); ct != nil && ct.Value != nil {
		resp["content_type"] = ct.Value
	}
	if headers := core.FindConnection("headers", inputs); headers != nil && headers.Value != nil {
		resp["headers"] = headers.Value
	}

	flow.SetOutput(WebResponseKey, resp)

	return map[string]interface{}{"set": true}, nil
}
