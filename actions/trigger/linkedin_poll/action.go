package linkedin_poll

import (
	core "flomation.app/automate/executor"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "LinkedIn Activity Trigger"
	Description  = "Triggers a flow on new comments or reactions on LinkedIn posts"
	Website      = "https://www.flomation.co"
	Icon         = "linkedin"
	Date         = "24/05/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "LinkedIn Access Token", Placeholder: "${credentials.linkedin}", Required: true},
	{Name: "post_urn", Type: core.ConnectionTypeString, Label: "Post URN", Placeholder: "urn:li:share:12345", Required: true},
	{Name: "event_filter", Type: core.ConnectionTypeString, Label: "Event Filter", Placeholder: "comment,reaction"},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval (seconds)", Placeholder: "300"},
}

var Outputs = [...]core.Connection{
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type"},
	{Name: "post_urn", Type: core.ConnectionTypeString, Label: "Post URN"},
	{Name: "author_urn", Type: core.ConnectionTypeString, Label: "Author URN"},
	{Name: "author_name", Type: core.ConnectionTypeString, Label: "Author Name"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content"},
	{Name: "comment_urn", Type: core.ConnectionTypeString, Label: "Comment URN"},
	{Name: "reaction_type", Type: core.ConnectionTypeString, Label: "Reaction Type"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing LinkedIn activity trigger")

	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
