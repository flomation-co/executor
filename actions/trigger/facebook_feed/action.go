package facebook_feed

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Page Feed Trigger"
	Description  = "Triggers a flow on comments, reactions or posts on a Facebook Page"
	Website      = "https://www.flomation.co"
	Icon         = "facebook"
	Date         = "24/05/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Facebook Page ID", Placeholder: "Your Facebook Page ID", Required: true},
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Facebook User Token", Placeholder: "${credentials.facebook}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "comment,reaction,post,share"},
}

var Outputs = [...]core.Connection{
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "verb", Type: core.ConnectionTypeString, Label: "Verb"},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID"},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Parent Post ID"},
	{Name: "post_id", Type: core.ConnectionTypeString, Label: "Post ID"},
	{Name: "sender_id", Type: core.ConnectionTypeString, Label: "Sender ID"},
	{Name: "sender_name", Type: core.ConnectionTypeString, Label: "Sender Name"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Content"},
	{Name: "reaction_type", Type: core.ConnectionTypeString, Label: "Reaction Type"},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID"},
	{Name: "page_access_token", Type: core.ConnectionTypeString, Label: "Page Access Token"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing Facebook feed trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
