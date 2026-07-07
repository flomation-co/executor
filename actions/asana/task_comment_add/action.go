package asana_task_comment_add

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Add Comment"
	Description  = "Add a comment to an Asana task."
	Website      = "https://www.flomation.co"
	Icon         = "asana+comment"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "The task to comment on", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "The comment text", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Comment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := asana.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	taskID, err := asana.RequiredString("task_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	text, err := asana.RequiredString("text", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"text": text}
	obj, err := asana.WriteObject(auth, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/stories", body, url.Values{})
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, "Added comment to task "+taskID), nil
}
