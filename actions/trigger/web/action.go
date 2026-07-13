package web

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Web Trigger"
	Description  = "Triggers a flow from an HTTP request of any verb; pair with a Web Response action to return a body/status"
	Website      = "https://www.flomation.co"
	Icon         = "globe"
	Date         = "13/07/2026"
	Type         = core.ActionTypeTrigger
)

// Inputs are the trigger's CONFIG (edited on the node), not the request data.
//   - methods: the accepted HTTP verbs (e.g. "POST" or "GET,POST"); a request
//     with another verb is rejected 405 by the edge before the flow runs.
//   - fields: a JSON map declaring where each request field comes from
//     (path/query/header/body) — the API resolves it and passes the values in
//     as the trigger's runtime outputs (referenced bare, e.g. ${id}, ${message}).
var Inputs = [...]core.Connection{
	{Name: "methods", Type: core.ConnectionTypeMultiSelect, Label: "Accepted Methods", Options: []core.ConnectionOption{
		{Name: "GET", Value: "GET"},
		{Name: "POST", Value: "POST"},
		{Name: "PUT", Value: "PUT"},
		{Name: "PATCH", Value: "PATCH"},
		{Name: "DELETE", Value: "DELETE"},
	}},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Request Fields", Placeholder: `{"id":"path","limit":"query","name":"body"}`},
	// auth is the optional gate on the invoke. "none" (default): public — callable
	// by anyone with the flow id. "publishable": require an embed publishable key +
	// allowed origin + resource opt-in (the embed-app gate). Identity via a
	// forwarded Sentinel JWT (${user.X}) works in both modes.
	{Name: "auth", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "Public (no key)", Value: "none"},
		{Name: "Publishable Key", Value: "publishable"},
	}},
	// keep_history turns this into a conversational endpoint: the invoke mints/
	// resumes a thread, injects prior turns as ${history}, and records both turns.
	// Off ⇒ a stateless API endpoint. message_field names the input treated as the
	// user's message for the recorded turn (default "message").
	{Name: "keep_history", Type: core.ConnectionTypeBoolean, Label: "Keep Conversation History"},
	{Name: "message_field", Type: core.ConnectionTypeString, Label: "Message Field", Placeholder: "message"},
}

// Outputs are the baseline request context always available as bare variables.
// Declared request fields (${id}, ${message}, …) and ${user.X}/identity are added
// by the API at dispatch, on top of these.
var Outputs = [...]core.Connection{
	{Name: "method", Type: core.ConnectionTypeString, Label: "HTTP Method"},
	{Name: "history", Type: core.ConnectionTypeString, Label: "Conversation History"},
	{Name: "raw_body", Type: core.ConnectionTypeString, Label: "Raw Body"},
}

// Execute surfaces the trigger data the API populated from the HTTP request
// (method, declared request fields, history, raw body) as this node's outputs,
// so they are referenceable bare (${method}, ${id}, ${history}, …). Mirrors the
// form trigger: the request context arrives as inputs and is echoed as outputs.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing web trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
