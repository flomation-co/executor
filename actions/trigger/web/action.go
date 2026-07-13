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
	{Name: "methods", Type: core.ConnectionTypeString, Label: "Accepted Methods", Placeholder: "POST or GET,POST"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Request Fields", Placeholder: `{"id":"path","limit":"query","name":"body"}`},
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
